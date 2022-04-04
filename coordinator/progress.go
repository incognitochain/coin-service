package coordinator

import "github.com/mongodb/mongo-tools/common/progress"

func (pm *ProgressManager) Attach(name string, progressor progress.Progressor) {
	pm.Progress = progressor
}

func (pm *ProgressManager) Detach(name string) {
	pm.Progress = nil
}

func (pm *ProgressManager) GetProgressStatus() (int64, int64) {
	return pm.Progress.Progress()
}
