package coordinator

import (
	"fmt"

	"github.com/mongodb/mongo-tools/common/progress"
)

func (pm *ProgressManager) Attach(name string, progressor progress.Progressor) {
	fmt.Println("Attach", name)
	pm.ProgressLock.Lock()
	defer pm.ProgressLock.Unlock()
	pm.Progress[name] = progressor
}

func (pm *ProgressManager) Detach(name string) {
	// pm.Progress = nil
}

func (pm *ProgressManager) GetProgressStatus() (int64, int64) {
	pm.ProgressLock.RLock()
	defer pm.ProgressLock.RUnlock()
	var current, max int64
	for _, v := range pm.Progress {
		c, m := v.Progress()
		current += c
		max += m
	}
	return current, max
}
