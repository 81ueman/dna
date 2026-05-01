package reachability

import (
	"fmt"
	"sort"

	"github.com/81ueman/dna/internal/model"
)

type ChangeKind string

const (
	ChangeAdd    ChangeKind = "+"
	ChangeRemove ChangeKind = "-"
)

type Change struct {
	Kind  ChangeKind
	Reach model.Reach
}

func Diff(oldReaches, newReaches []model.Reach) []Change {
	oldSet := reachSet(oldReaches)
	newSet := reachSet(newReaches)

	var changes []Change
	for reach := range newSet {
		if !oldSet[reach] {
			changes = append(changes, Change{Kind: ChangeAdd, Reach: reach})
		}
	}
	for reach := range oldSet {
		if !newSet[reach] {
			changes = append(changes, Change{Kind: ChangeRemove, Reach: reach})
		}
	}

	sortChanges(changes)
	return changes
}

func FormatChange(change Change) string {
	return fmt.Sprintf(
		"%sReach(%s,%s,%s,%s)",
		change.Kind,
		change.Reach.Source,
		change.Reach.Dest,
		change.Reach.VRF,
		change.Reach.Prefix.Masked(),
	)
}

func reachSet(reaches []model.Reach) map[model.Reach]bool {
	set := make(map[model.Reach]bool, len(reaches))
	for _, reach := range reaches {
		reach.Prefix = reach.Prefix.Masked()
		set[reach] = true
	}
	return set
}

func sortChanges(changes []Change) {
	sort.Slice(changes, func(i, j int) bool {
		a, b := changes[i], changes[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Reach.Source != b.Reach.Source {
			return a.Reach.Source < b.Reach.Source
		}
		if a.Reach.Dest != b.Reach.Dest {
			return a.Reach.Dest < b.Reach.Dest
		}
		if a.Reach.VRF != b.Reach.VRF {
			return a.Reach.VRF < b.Reach.VRF
		}
		return comparePrefix(a.Reach.Prefix, b.Reach.Prefix) < 0
	})
}
