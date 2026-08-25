package cabinet

import (
	"fmt"
	"time"
)

func formatSequence(sequence uint64) string { return fmt.Sprintf("%06d", sequence) }
func timePtr(value time.Time) *time.Time    { return &value }
