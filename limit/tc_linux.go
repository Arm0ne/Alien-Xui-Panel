//go:build linux

package limit

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"x-ui/logger"
)

const (
	rootHandle  = "1:"
	filterPrio  = "10"
	filterProto = "ip"
	// unlimitedRateMbit 是「不限速」默认 class 的速率。
	// 只要远大于真实网卡带宽就等于不限速：25Gbit 高于常见的 1G/2.5G/10G 网卡，
	// 又低于 tc 传统上需要 64 位扩展才能表达的约 34Gbit 上限，兼容性最好。
	// 不再去读 /sys/class/net/<网卡>/speed，避免虚拟网卡上报异常值把不限速流量压住。
	unlimitedRateMbit = 25000
)

var (
	mu           sync.Mutex
	applied      *Plan
	appliedIface string
	setupDone    bool
)

// Sync 把当前期望的限速状态下发到内核。
// entries 为空表示不再限制任何入站，此时会清理面板之前下发的规则。
func Sync(entries []Entry) (*Report, error) {
	plan := BuildPlan(entries)
	report := &Report{Warnings: plan.Warnings}

	mu.Lock()
	defer mu.Unlock()

	if plan.Empty() {
		if !setupDone {
			return report, nil
		}
		logger.Infof("[限速] 已没有需要限速的入站，清理 %s 上的 tc 规则", appliedIface)
		teardown(appliedIface)
		applied, appliedIface, setupDone = nil, "", false
		report.Changed = true
		return report, nil
	}

	iface, err := defaultRouteIface()
	if err != nil {
		return report, err
	}
	report.Iface = iface
	report.Rules = len(plan.Rules)
	report.Rates = plan.Rates

	switch {
	case !setupDone || appliedIface != iface || !sameRates(applied, plan):
		// 网卡变了或限速档位变了：重建 qdisc / class，再重下 filter
		if err := setup(iface, plan); err != nil {
			applied, appliedIface, setupDone = nil, "", false
			return report, err
		}
	case sameRules(applied, plan):
		// 没有任何变化，什么都不用做
		return report, nil
	}

	if err := applyFilters(iface, plan); err != nil {
		applied, appliedIface, setupDone = nil, "", false
		return report, err
	}

	applied = plan
	appliedIface = iface
	setupDone = true
	report.Changed = true
	logger.Infof("[限速] 已在 %s 下发 %d 条入站限速: %s", iface, len(plan.Rules), plan.Describe())
	return report, nil
}

// Clear 清理面板下发的所有限速规则（面板退出时调用）。
func Clear() error {
	mu.Lock()
	defer mu.Unlock()

	if !setupDone || appliedIface == "" {
		return nil
	}
	logger.Infof("[限速] 正在清理 %s 上的 tc 限速规则", appliedIface)
	teardown(appliedIface)
	applied, appliedIface, setupDone = nil, "", false
	return nil
}

// setup 建立根 qdisc、默认（不限速）class 以及每个限速档位对应的 class。
func setup(iface string, plan *Plan) error {
	linkRate := strconv.Itoa(unlimitedRateMbit) + "mbit"
	defaultClass := rootHandle + strconv.Itoa(defaultClassId)

	qdiscCmds := [][]string{
		{"qdisc", "replace", "dev", iface, "root", "handle", rootHandle, "htb", "default", strconv.Itoa(defaultClassId)},
		{"class", "replace", "dev", iface, "parent", rootHandle, "classid", defaultClass,
			"htb", "rate", linkRate, "ceil", linkRate, "burst", "1m", "cburst", "1m"},
	}
	for _, cmd := range qdiscCmds {
		if err := run("tc", cmd...); err != nil {
			return fmt.Errorf("建立限速队列失败(需要 tc/iproute2 与 root 权限): %w", err)
		}
	}
	// 叶子队列用 fq_codel 可以降低延迟，失败也不影响限速本身
	runIgnoreErr("tc", "qdisc", "replace", "dev", iface, "parent", defaultClass, "fq_codel")

	for _, rate := range plan.Rates {
		classId := plan.ClassOfRate[rate]
		rateStr := strconv.Itoa(rate) + "mbit"
		burst := burstFor(rate)
		classIdStr := rootHandle + strconv.Itoa(classId)

		if err := run("tc", "class", "replace", "dev", iface, "parent", rootHandle, "classid", classIdStr,
			"htb", "rate", rateStr, "ceil", rateStr, "burst", burst, "cburst", burst); err != nil {
			return err
		}
		runIgnoreErr("tc", "qdisc", "replace", "dev", iface, "parent", classIdStr, "fq_codel")
	}
	return nil
}

// applyFilters 重新下发按端口的 filter（先清掉旧的，再全部添加）。
// 下行报文（服务器 -> 客户端）的源端口就是入站的监听端口。
func applyFilters(iface string, plan *Plan) error {
	runIgnoreErr("tc", "filter", "del", "dev", iface, "parent", rootHandle, "protocol", filterProto, "prio", filterPrio)

	for i, r := range plan.Rules {
		handle := "0x" + strconv.FormatInt(int64(i+1), 16)
		err := run("tc", "filter", "add", "dev", iface, "parent", rootHandle, "protocol", filterProto,
			"prio", filterPrio, "handle", handle,
			"u32", "match", "ip", "sport", strconv.Itoa(r.Port), "0xffff",
			"flowid", rootHandle+strconv.Itoa(r.ClassId))
		if err != nil {
			return err
		}
	}
	return nil
}

// teardown 删除面板自己建立的根 qdisc（连带其中的 class 与 filter）。
func teardown(iface string) {
	if iface == "" {
		return
	}
	runIgnoreErr("tc", "qdisc", "del", "dev", iface, "root")
}

// defaultRouteIface 读出默认路由所在的网卡，避免把网卡名写死成 eth0。
func defaultRouteIface() (string, error) {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return "", fmt.Errorf("无法读取 /proc/net/route: %w", err)
	}

	iface := ""
	bestMetric := -1
	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		// fields[1] 是 Destination，全 0 代表默认路由
		if fields[1] != "00000000" {
			continue
		}
		metric, _ := strconv.Atoi(fields[6])
		if iface == "" || metric < bestMetric {
			iface = fields[0]
			bestMetric = metric
		}
	}
	if iface == "" {
		return "", fmt.Errorf("未找到默认路由网卡，无法下发限速规则")
	}
	return iface, nil
}

// burstFor 按速率给出一个合适的突发额度（约 50ms 的数据量）。
// 不设置 burst 时 HTB 的默认值往往过小，实测会跑不满设定速率。
func burstFor(rateMbps int) string {
	bytes := rateMbps * 1000000 / 8 / 20
	if bytes < 16*1024 {
		bytes = 16 * 1024
	}
	if bytes > 512*1024 {
		bytes = 512 * 1024
	}
	return strconv.Itoa(bytes/1024) + "k"
}

func dryRun() bool {
	v := strings.TrimSpace(os.Getenv("XUI_TC_DRY_RUN"))
	return v == "1" || strings.EqualFold(v, "true")
}

// run 执行一条命令，失败时返回带完整命令与输出的错误。
func run(name string, args ...string) error {
	cmdline := name + " " + strings.Join(args, " ")
	if dryRun() {
		logger.Infof("[限速][dry-run] %s", cmdline)
		return nil
	}
	logger.Debugf("[限速] 执行: %s", cmdline)
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v %s", cmdline, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runIgnoreErr 执行一条允许失败的命令（例如清理不存在的规则）。
func runIgnoreErr(name string, args ...string) {
	if err := run(name, args...); err != nil {
		logger.Debugf("[限速] 忽略失败: %v", err)
	}
}
