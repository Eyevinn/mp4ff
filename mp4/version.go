package mp4

import (
	"fmt"
	"strconv"
	"time"
)

var (
	commitVersion string
	commitDate    string
)

// GetVersion - get version and also commitHash and commitDate if inserted
func GetVersion() string {
	seconds, _ := strconv.Atoi(commitDate)
	if commitDate != "" {
		t := time.Unix(int64(seconds), 0)
		return fmt.Sprintf("%s, date: %s", commitVersion, t.Format("2006-01-02"))
	}
	return commitVersion
}
