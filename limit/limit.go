// Package limit 实现「按入站端口限速」。
//
// 背景（为什么需要它）:
// Xray-core 的 policy.levels 里只有「连接超时」和「统计开关」两类字段，
// 其中 uplinkOnly / downlinkOnly 的单位是「秒」（见 xray-core 的
// infra/conf/policy.go 与 features/policy/policy.go），并不是带宽限制。
// 因此把 speedLimit 当成 level 下发给 Xray 的做法不可能限速。
// 真正生效的做法是在内核层用 tc 的 HTB 队列对网卡流量整形，
// 本包负责把「入站端口 -> 限速值」翻译成 tc 的 class / filter 并下发。
//
// 粒度说明:
// 限速作用在「入站端口」上，即同一个入站下的所有用户共享同一个带宽额度。
// 方向说明:
// 只限制下行（服务器 -> 客户端），匹配的是「源端口 = 入站端口」的报文，
// 与手工执行的 `tc filter ... match ip sport <入站端口>` 行为一致。
package limit

import (
	"fmt"
	"sort"
	"strings"
)

const (
	// defaultClassId 是 HTB 里「不限速」的默认 class 编号（其余流量都走它）。
	defaultClassId = 999
	// firstClassId 是第一个限速 class 的起始编号。
	firstClassId = 10
	// maxClassCount 保证限速 class 不会撞上默认 class。
	maxClassCount = defaultClassId - firstClassId
)

// Entry 是一个「需要限速的入站」。
type Entry struct {
	InboundId int
	Remark    string
	Port      int
	RateMbps  int
}

// Rule 是一条最终下发给 tc 的限速规则。
type Rule struct {
	Port     int
	RateMbps int
	ClassId  int
}

// Plan 是本次要下发的完整规则集合。
type Plan struct {
	// Rates 是去重并升序排列后的限速值；每个限速值对应一个 tc class。
	Rates []int
	// ClassOfRate 记录限速值到 tc class 编号的映射。
	ClassOfRate map[int]int
	// Rules 按端口升序排列。
	Rules    []Rule
	Warnings []string
}

// Empty 表示当前没有任何入站需要限速。
func (p *Plan) Empty() bool {
	return p == nil || len(p.Rules) == 0
}

// Describe 生成便于阅读的摘要，例如 "端口 43667 -> 30Mbps"。
func (p *Plan) Describe() string {
	if p.Empty() {
		return "无"
	}
	parts := make([]string, 0, len(p.Rules))
	for _, r := range p.Rules {
		parts = append(parts, fmt.Sprintf("端口 %d -> %dMbps", r.Port, r.RateMbps))
	}
	return strings.Join(parts, ", ")
}

// Report 是一次下发的执行结果，供日志使用。
type Report struct {
	Iface    string
	Rules    int
	Rates    []int
	Changed  bool
	Warnings []string
}

// BuildPlan 把入站配置整理成要去重的规则集合。
// 同一个端口出现多次时取最严格（最小）的限速值，并记录一条警告。
func BuildPlan(entries []Entry) *Plan {
	plan := &Plan{ClassOfRate: make(map[int]int)}

	rateSet := make(map[int]bool)
	portRate := make(map[int]int)
	portRemark := make(map[int]string)
	portCount := make(map[int]int)

	for _, e := range entries {
		if e.Port <= 0 {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("忽略端口非法的入站(id=%d, port=%d)", e.InboundId, e.Port))
			continue
		}
		if e.RateMbps <= 0 {
			continue
		}
		portCount[e.Port]++
		old, exists := portRate[e.Port]
		if !exists || e.RateMbps < old {
			portRate[e.Port] = e.RateMbps
		}
		if e.Remark != "" {
			portRemark[e.Port] = e.Remark
		}
		rateSet[e.RateMbps] = true
	}

	for port, count := range portCount {
		if count > 1 {
			plan.Warnings = append(plan.Warnings,
				fmt.Sprintf("端口 %d 有 %d 个入站同时限速，已按最严格的值 %dMbps 处理", port, count, portRate[port]))
		}
	}

	rates := make([]int, 0, len(rateSet))
	for r := range rateSet {
		rates = append(rates, r)
	}
	sort.Ints(rates)

	if len(rates) > maxClassCount {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("限速档位过多(%d)，仅下发前 %d 档", len(rates), maxClassCount))
		rates = rates[:maxClassCount]
	}
	plan.Rates = rates

	for i, r := range rates {
		plan.ClassOfRate[r] = firstClassId + i
	}

	ports := make([]int, 0, len(portRate))
	for port := range portRate {
		ports = append(ports, port)
	}
	sort.Ints(ports)

	for _, port := range ports {
		rate := portRate[port]
		classId, ok := plan.ClassOfRate[rate]
		if !ok {
			// 超过档位上限时忽略该端口
			continue
		}
		plan.Rules = append(plan.Rules, Rule{Port: port, RateMbps: rate, ClassId: classId})
	}

	return plan
}

// sameRates 判断两次下发的限速档位是否一致（不一致需要重建 class）。
func sameRates(a, b *Plan) bool {
	if len(planRates(a)) != len(planRates(b)) {
		return false
	}
	for i, r := range planRates(a) {
		if r != planRates(b)[i] {
			return false
		}
	}
	return true
}

// sameRules 判断两次下发的端口规则是否一致（一致则无需改动 filter）。
func sameRules(a, b *Plan) bool {
	ar, br := planRules(a), planRules(b)
	if len(ar) != len(br) {
		return false
	}
	for i := range ar {
		if ar[i] != br[i] {
			return false
		}
	}
	return true
}

func planRates(p *Plan) []int {
	if p == nil {
		return nil
	}
	return p.Rates
}

func planRules(p *Plan) []Rule {
	if p == nil {
		return nil
	}
	return p.Rules
}
