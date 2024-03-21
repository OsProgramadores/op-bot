package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// opMatchAction defines the possible actions to take when a pattern matches.
type opMatchAction int64

const (
	// opNoAction indicates no specific action should be taken.
	opNoAction opMatchAction = iota
	// Ban indicates the user should be banned.
	opBan
	// Kick indicates the user should be kicked.
	opKick

	// File to store the list of patterns.
	patternsFile = "patterns.toml"
)

// opPatternAction contains a pattern and its associated action, in string form.
type opPatternAction struct {
	Pattern string `toml:"pattern"`
	Action  string `toml:"action"`
}

// opPatterns contains the lists of patterns to match against.
type opPatterns struct {
	Nickname []opPatternAction `toml:"nickname"`
	Username []opPatternAction `toml:"username"`
	Bio      []opPatternAction `toml:"bio"`
	Message  []opPatternAction `toml:"message"`
}

// opMatchPattern contains the data we will use when matching.
type opMatchPattern struct {
	// These three items will be matched when a new user joins
	// It's a more expensive operation because we need to perform
	// a web request to get additional info.
	Nickname string // This is the first + last name.
	Username string
	Bio      string
	// This will be matched in regular messages.
	Message string
}

// This is a way to obtain specific information not available with the current
// bot api we are using; instead, we will do the HTTP request ourselves and
// unmarshal its result in this struct.
type opBotUserInfo struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	UserName  string `json:"username,omitempty"`
	Bio       string `json:"bio,omitempty"`
}

// actionFromString() returns an opMatchAction from a string input;
// default is NoAction.
func actionFromString(s string) opMatchAction {
	actions := map[string]opMatchAction{
		"kick": opKick,
		"ban":  opBan,
	}

	if action, ok := actions[strings.ToLower(s)]; ok {
		return action
	}
	return opNoAction
}

// String returns the MatchAction as a string.
func (ma opMatchAction) String() string {
	actions := map[opMatchAction]string{
		opKick: "Kick",
		opBan:  "Ban",
	}
	if action, ok := actions[ma]; ok {
		return action
	}
	return "NoAction"
}

// getMatchPattern() gets the relevant data from the update message.
// For messages indicating new users have joined, it performs a web request to
// get additional info on the user; for regular messages, we get the actual
// message sent to use when matching.
func getMatchPattern(bot *tgbotapi.BotAPI, update tgbotapi.Update) (opMatchPattern, error) {
	matchPattern := opMatchPattern{}
	switch {
	case update.Message == nil:
		return opMatchPattern{}, fmt.Errorf("getMatchPattern:  Invalid message")
	case update.Message.NewChatMembers != nil:
		// When a new user joins, we get some info on him/her to match against.
		args := url.Values{}
		args.Add("chat_id", strconv.FormatInt(int64(update.Message.From.ID), 10))

		// Note that we are using bot.MakeRequest directly because some of the
		// fields we care about are not available through the current API we are
		// using, so we get those directly from the getChat HTTP request instead.
		// This may be improved/optimized in the future, if we move to a newer
		// API release.
		resp, err := bot.MakeRequest("getChat", args)
		if err != nil {
			return opMatchPattern{}, err
		}
		fmt.Printf("getMatchPattern: %+v\n", resp)

		// We first unmarshal the results into this struct to get the data we
		// want. Later we will build a slightly different object which is what
		// we will actually use.
		var userinfo opBotUserInfo
		err = json.Unmarshal(resp.Result, &userinfo)
		if err != nil {
			return opMatchPattern{}, err
		}

		// Now we construct a MatchPattern, which basically has the Nickname
		// as being first + last name, to simplify the match further on.
		matchPattern.Nickname = strings.Trim(fmt.Sprintf("%s %s", userinfo.FirstName, userinfo.LastName), " ")
		matchPattern.Username = userinfo.UserName
		matchPattern.Bio = strings.Trim(userinfo.Bio, " ")
	default:
		// By default, we only get the actual message to match against.
		matchPattern.Message = update.Message.Text
	}
	return matchPattern, nil
}

// stringTomlToPatterns() converts the toml patterns from string to the Patterns
// type, that ca be used for the matching.
func stringTomlToPatterns(sp string) (opPatterns, error) {
	newPatterns := opPatterns{}
	if _, err := toml.Decode(sp, &newPatterns); err != nil {
		return opPatterns{}, err
	}
	return newPatterns, nil
}

// loadPatterns() reload the patterns file from the disk.
func loadPatterns() (opPatterns, error) {
	cfgdir, err := configDir()
	if err != nil {
		return opPatterns{}, err
	}

	f := filepath.Join(cfgdir, patternsFile)

	buf, err := os.ReadFile(f)
	if err != nil {
		return opPatterns{}, err
	}

	return stringTomlToPatterns(string(buf))
}

// performMatch() performs a simple regex match operation using the given
// pattern and data.
func performMatch(pattern, data string) bool {
	// Note that we add the "(?i)" flag to have a case-insensitive match.
	r, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		fmt.Printf("performMatch: unable to compile pattern %q: %v\n", pattern, err)
		return false
	}
	return r.MatchString(data)
}

// performGroupMatch() performs a series of regex match operations with the
// provided patterns/action and data.
func performGroupMatch(patterns []opPatternAction, data string) (bool, opMatchAction) {
	if len(data) == 0 {
		return false, opNoAction
	}

	for _, ma := range patterns {
		if len(ma.Pattern) == 0 {
			continue
		}
		if performMatch(ma.Pattern, data) {
			return true, actionFromString(ma.Action)
		}
	}
	return false, opNoAction
}

// matchPattern() performs the actual pattern matching using the data
// we have and the list of patterns to match against.
func (p *opPatterns) matchPattern(m opMatchPattern) (bool, opMatchAction) {
	if len(m.Message) > 0 {
		// Common case; match against actual message.
		return performGroupMatch(p.Message, m.Message)
	}

	// We are matching against the data from a new user who just joined.

	// The order we match is: 1. Bio, 2. Username, 3. Nickname. The
	// order matters because more than one attribute may match, but
	// once a match happens, we already return, so pay attention
	// when writing the pattern rules.
	if len(m.Bio) > 0 {
		if ret, action := performGroupMatch(p.Bio, m.Bio); ret {
			return ret, action
		}
	}
	if len(m.Username) > 0 {
		if ret, action := performGroupMatch(p.Username, m.Username); ret {
			return ret, action
		}
	}
	if len(m.Nickname) > 0 {
		if ret, action := performGroupMatch(p.Nickname, m.Nickname); ret {
			return ret, action
		}
	}

	return false, opNoAction
}

// MatchFromUpdate constructs a MatchPattern from the update message and call
// matchPattern() to do the actual matching.  This is for gluing the bot with
// the actual matching, while making the matching itself more testable.
func (p *opPatterns) MatchFromUpdate(b *tgbotapi.BotAPI, u tgbotapi.Update) (bool, opMatchAction) {
	if u.Message == nil || u.Message.Chat == nil {
		return false, opNoAction
	}

	mp, err := getMatchPattern(b, u)
	if err != nil {
		return false, opNoAction
	}

	return p.matchPattern(mp)
}
