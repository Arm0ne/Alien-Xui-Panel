//go:build !linux

package limit

import (
	"sync"

	"x-ui/logger"
)

var warnOnce sync.Once

// Sync 在非 Linux 系统上不做任何事，只提示一次。
func Sync(entries []Entry) (*Report, error) {
	if !BuildPlan(entries).Empty() {
		warnOnce.Do(func() {
			logger.Warning("[限速] 入站限速依赖 Linux 的 tc/HTB，当前系统不会生效")
		})
	}
	return &Report{}, nil
}

// Clear 在非 Linux 系统上无需清理。
func Clear() error {
	return nil
}
