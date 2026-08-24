package compute

import (
	"fmt"
)

func fmtSignature(taskID string, total int) string {
	return fmt.Sprintf("%s#%d", taskID, total)
}
