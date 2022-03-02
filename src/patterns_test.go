// Unit tests for the patterns module.
package main

import (
	"testing"
)

const (
	patterns = `[[nickname]]
pattern = "^BAD"
action = "kick"

[[nickname]]
pattern = "foo.*"
action = "ban"

[[username]]
pattern = "^foobar.*"
action = "kick"

[[bio]]
pattern = "fo.o.*b.aar"
action = "ban"

[[message]]
pattern = "foobar.example.com"
action = "ban"`
)

func TestPatternMatch(t *testing.T) {
	caseTests := []struct {
		mp             opMatchPattern // data to be matched agasint
		expected       bool           // Result expected.
		expectedAction opMatchAction  // Action expected
	}{
		{
			// zero-value opMatchPattern.
			mp:             opMatchPattern{},
			expected:       false,
			expectedAction: opNoAction,
		},
		{
			// Regular message.
			mp: opMatchPattern{
				Message: "Access this new website: foobar.example.com!!!",
			},
			expected:       true,
			expectedAction: opBan,
		},
		{
			mp: opMatchPattern{
				Username: "  foobar42",
			},
			expected:       false,
			expectedAction: opNoAction,
		},
		{
			mp: opMatchPattern{
				Username: "foobar42",
			},
			expected:       true,
			expectedAction: opKick,
		},
		{
			// Username will match and Bio will not.
			mp: opMatchPattern{
				Username: "foobar42",
				Bio:      "fooooooooooooooooooo bar",
			},
			expected:       true,
			expectedAction: opKick,
		},
		{
			// Both Username and Bio will match, but Bio is evaluated first.
			mp: opMatchPattern{
				Username: "foobar42",
				Bio:      "fooooooooooooooooooo bbaar",
			},
			expected:       true,
			expectedAction: opBan,
		},
		{
			mp: opMatchPattern{
				Nickname: "FOO baar",
			},
			expected:       true,
			expectedAction: opBan,
		},
		{
			mp: opMatchPattern{
				Nickname: "Just another BADWORD name",
			},
			expected:       false,
			expectedAction: opNoAction,
		},
		{
			mp: opMatchPattern{
				Nickname: "BADWORD name",
			},
			expected:       true,
			expectedAction: opKick,
		},
	}

	p, _ := stringTomlToPatterns(patterns)

	for _, tt := range caseTests {
		ret, action := p.matchPattern(tt.mp)
		if ret != tt.expected || action != tt.expectedAction {
			t.Errorf("matchPattern handled %v incorrectly; expected ret: %v, got: %v, expected action: %v, got: %v", tt.mp, tt.expected, ret, tt.expectedAction, action)
		}
	}
}
