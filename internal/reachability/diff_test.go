package reachability

import (
	"net/netip"
	"testing"

	"github.com/81ueman/dna/internal/model"
)

func TestDiffReturnsAddedReachability(t *testing.T) {
	changes := Diff(nil, []model.Reach{
		reach("h1", "h2", model.DefaultVRF, "10.0.2.0/24"),
	})

	assertEqual(t, changes, []Change{
		{Kind: ChangeAdd, Reach: reach("h1", "h2", model.DefaultVRF, "10.0.2.0/24")},
	})
}

func TestDiffReturnsRemovedReachability(t *testing.T) {
	changes := Diff(
		[]model.Reach{
			reach("h1", "h2", model.DefaultVRF, "10.0.2.0/24"),
		},
		nil,
	)

	assertEqual(t, changes, []Change{
		{Kind: ChangeRemove, Reach: reach("h1", "h2", model.DefaultVRF, "10.0.2.0/24")},
	})
}

func TestDiffSuppressesUnchangedReachability(t *testing.T) {
	oldReaches := []model.Reach{
		reach("h1", "h2", model.DefaultVRF, "10.0.2.0/24"),
	}
	newReaches := []model.Reach{
		reach("h1", "h2", model.DefaultVRF, "10.0.2.0/24"),
	}

	changes := Diff(oldReaches, newReaches)

	if len(changes) != 0 {
		t.Fatalf("changes = %#v, want none", changes)
	}
}

func TestDiffSortsMixedChanges(t *testing.T) {
	changes := Diff(
		[]model.Reach{
			reach("z", "a", model.DefaultVRF, "10.0.4.0/24"),
			reach("a", "z", "blue", "10.0.1.0/24"),
		},
		[]model.Reach{
			reach("a", "m", model.DefaultVRF, "10.0.3.0/24"),
			reach("a", "m", model.DefaultVRF, "10.0.2.0/24"),
		},
	)

	assertEqual(t, changes, []Change{
		{Kind: ChangeAdd, Reach: reach("a", "m", model.DefaultVRF, "10.0.2.0/24")},
		{Kind: ChangeAdd, Reach: reach("a", "m", model.DefaultVRF, "10.0.3.0/24")},
		{Kind: ChangeRemove, Reach: reach("a", "z", "blue", "10.0.1.0/24")},
		{Kind: ChangeRemove, Reach: reach("z", "a", model.DefaultVRF, "10.0.4.0/24")},
	})
}

func TestDiffDeduplicatesAndMasksInput(t *testing.T) {
	oldReaches := []model.Reach{
		reach("h1", "h2", model.DefaultVRF, "10.0.2.42/24"),
		reach("h1", "h2", model.DefaultVRF, "10.0.2.0/24"),
	}
	newReaches := []model.Reach{
		reach("h1", "h2", model.DefaultVRF, "10.0.3.42/24"),
		reach("h1", "h2", model.DefaultVRF, "10.0.3.0/24"),
	}

	changes := Diff(oldReaches, newReaches)

	assertEqual(t, changes, []Change{
		{Kind: ChangeAdd, Reach: reach("h1", "h2", model.DefaultVRF, "10.0.3.0/24")},
		{Kind: ChangeRemove, Reach: reach("h1", "h2", model.DefaultVRF, "10.0.2.0/24")},
	})
}

func TestFormatChange(t *testing.T) {
	got := FormatChange(Change{
		Kind:  ChangeAdd,
		Reach: reach("h1", "h2", model.DefaultVRF, "10.0.2.42/24"),
	})

	const want = "+Reach(h1,h2,default,10.0.2.0/24)"
	if got != want {
		t.Fatalf("FormatChange = %q, want %q", got, want)
	}
}

func reach(source, dest model.EdgePortID, vrf model.VRF, prefix string) model.Reach {
	return model.Reach{
		Source: source,
		Dest:   dest,
		VRF:    vrf,
		Prefix: netip.MustParsePrefix(prefix).Masked(),
	}
}
