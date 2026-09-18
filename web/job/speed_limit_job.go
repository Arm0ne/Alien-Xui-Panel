package job

import (
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/limit"
	"x-ui/logger"
)

// SpeedLimitJob 负责把数据库里的「入站限速」同步到内核的 tc 规则上。
//
// 说明:
//   - Xray-core 自身没有限速能力，真正的限速由 tc(HTB) 完成（见 limit 包）；
//   - 这里只做「对账」：配置变了才下发命令，没变就什么都不做；
//   - 面板启动、Xray 重启、网卡变化、手工改过 tc 规则等情况都会被下一次对账纠正。
type SpeedLimitJob struct {
	lastErr string
}

func NewSpeedLimitJob() *SpeedLimitJob {
	return &SpeedLimitJob{}
}

func (j *SpeedLimitJob) Run() {
	entries, err := j.collectEntries()
	if err != nil {
		j.reportError(err)
		return
	}

	report, err := limit.Sync(entries)
	if err != nil {
		j.reportError(err)
		return
	}

	if j.lastErr != "" {
		j.lastErr = ""
		logger.Info("[限速] tc 规则同步已恢复正常")
	}
	if report.Changed {
		for _, w := range report.Warnings {
			logger.Warningf("[限速] %s", w)
		}
	}
}

// collectEntries 取出所有「已启用且设置了限速」的入站。
func (j *SpeedLimitJob) collectEntries() ([]limit.Entry, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Model(model.Inbound{}).Where("enable = ?", true).Find(&inbounds).Error; err != nil {
		return nil, err
	}

	entries := make([]limit.Entry, 0, len(inbounds))
	for _, inbound := range inbounds {
		if inbound == nil || inbound.SpeedLimit <= 0 {
			continue
		}
		entries = append(entries, limit.Entry{
			InboundId: inbound.Id,
			Remark:    inbound.Remark,
			Port:      inbound.Port,
			RateMbps:  inbound.SpeedLimit,
		})
	}
	return entries, nil
}

// reportError 只在错误内容变化时打日志，避免每 10 秒刷屏。
func (j *SpeedLimitJob) reportError(err error) {
	if err == nil {
		return
	}
	msg := err.Error()
	if msg == j.lastErr {
		return
	}
	j.lastErr = msg
	logger.Warningf("[限速] %v", err)
}
