package gerrit

import (
	"testing"

	goGerrit "github.com/andygrunwald/go-gerrit"
	"github.com/stretchr/testify/assert"
)

func TestIsAutosubmit(t *testing.T) {
	emptyChangeInfo := &goGerrit.ChangeInfo{}
	assert.Equal(t, false, MakeChangeset(emptyChangeInfo).IsAutosubmit(), "A changeset without the Autosubmit label present shouldn't be autosubmittable")

	// +1
	changeInfoWithAutosubmitLabelSet := &goGerrit.ChangeInfo{
		Labels: map[string]goGerrit.LabelInfo{
			"Autosubmit": {
				Approved: goGerrit.AccountInfo{AccountID: 1},
			},
		},
	}
	assert.Equal(t, true, MakeChangeset(changeInfoWithAutosubmitLabelSet).IsAutosubmit(), "Autosubmit label set to +1 should be autosubmittable")

	// ensure some common label values don't trigger autosubmit. We only trigger on a "+1"

	// +2
	changeInfoWithAutosubmitLabelSetToPlusTwo := &goGerrit.ChangeInfo{
		Labels: map[string]goGerrit.LabelInfo{
			"Autosubmit": {
				Recommended: goGerrit.AccountInfo{AccountID: 1},
			},
		},
	}
	assert.Equal(t, false, MakeChangeset(changeInfoWithAutosubmitLabelSetToPlusTwo).IsAutosubmit(), "Autosubmit label set to +2 should not be autosubmittable")

	// -1
	changeInfoWithAutosubmitLabelSetToMinusOne := &goGerrit.ChangeInfo{
		Labels: map[string]goGerrit.LabelInfo{
			"Autosubmit": {
				Disliked: goGerrit.AccountInfo{AccountID: 1},
			},
		},
	}
	assert.Equal(t, false, MakeChangeset(changeInfoWithAutosubmitLabelSetToMinusOne).IsAutosubmit(), "Autosubmit label set to -1 should not be autosubmittable")

	// -2
	changeInfoWithAutosubmitLabelSetToMinusTwo := &goGerrit.ChangeInfo{
		Labels: map[string]goGerrit.LabelInfo{
			"Autosubmit": {
				Rejected: goGerrit.AccountInfo{AccountID: 1},
			},
		},
	}
	assert.Equal(t, false, MakeChangeset(changeInfoWithAutosubmitLabelSetToMinusTwo).IsAutosubmit(), "Autosubmit label set to -2 should not be autosubmittable")
}
