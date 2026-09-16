package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// headerChecksPath must match /usr/local/bin/block-sender's own hardcoded
// path exactly — the dashboard and that CLI script manage the same file, in
// the same line format, so either one can be used interchangeably and the
// other still understands what's there.
const headerChecksPath = "/etc/postfix/header_checks"

// blockRuleRe matches a rule block-sender (or blockSender below) appends:
// `/^From:.*escaped\.domain/ REJECT Spammer blocked: original@domain`
// — group 2 is the original, unescaped address exactly as block-sender
// would have received it.
var blockRuleRe = regexp.MustCompile(`^/\^From:\.\*(.+)/ REJECT Spammer blocked: (.+)$`)
var addedCommentRe = regexp.MustCompile(`^# Added on (.+)$`)

// emailPatternRe is a permissive but not-empty check; the important
// constraint is no "/" (header_checks' regexp_table delimiter — an address
// containing one would prematurely close the pattern) and no whitespace
// (would break the space-separated REJECT rule syntax).
var emailPatternRe = regexp.MustCompile(`^[^\s/]+@[^\s/]+\.[^\s/]+$`)

type blockedSender struct {
	Email   string `json:"email"`
	AddedAt string `json:"addedAt"`
}

// escapeEmailForRegex mirrors block-sender's own `sed 's/\./\\./g'` —
// only dots are escaped (a literal match is still correct without escaping
// "@", which isn't a regex metacharacter).
func escapeEmailForRegex(email string) string {
	return strings.ReplaceAll(email, ".", `\.`)
}

func blockRuleLine(email string) string {
	return fmt.Sprintf("/^From:.*%s/ REJECT Spammer blocked: %s", escapeEmailForRegex(email), email)
}

// requireRoot is checked before any header_checks write — the systemd
// service runs mail-monitor as root (see ops/systemd/mail-monitor.service),
// but a developer running the binary directly won't have permission to
// edit /etc/postfix or reload postfix, and should get a clear error
// instead of a confusing permission-denied from deep inside a write.
func requireRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("root 권한이 필요합니다 (mail-monitor.service로 실행 중인지 확인하세요)")
	}
	return nil
}

func reloadPostfix() error {
	return exec.Command("postfix", "reload").Run()
}

// listBlockedSenders parses header_checks for every rule block-sender's
// format produced, pairing each with the "# Added on ..." comment line
// immediately above it when present.
func listBlockedSenders() ([]blockedSender, error) {
	data, err := os.ReadFile(headerChecksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []blockedSender{}, nil
		}
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	out := []blockedSender{}
	for i, line := range lines {
		m := blockRuleRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		addedAt := ""
		if i > 0 {
			if am := addedCommentRe.FindStringSubmatch(lines[i-1]); am != nil {
				addedAt = am[1]
			}
		}
		out = append(out, blockedSender{Email: m[2], AddedAt: addedAt})
	}
	return out, nil
}

// blockSender appends a block-sender-format rule for email, unless one
// already exists (matching block-sender's own duplicate check). Returns
// alreadyBlocked=true rather than an error in that case.
func blockSender(email string) (alreadyBlocked bool, err error) {
	if !emailPatternRe.MatchString(email) {
		return false, fmt.Errorf("올바른 이메일 형식이 아닙니다: %s", email)
	}
	if err := requireRoot(); err != nil {
		return false, err
	}

	rule := blockRuleLine(email)
	data, err := os.ReadFile(headerChecksPath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if strings.Contains(string(data), rule) {
		return true, nil
	}

	f, err := os.OpenFile(headerChecksPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	defer f.Close()
	ts := time.Now().Format("2006-01-02 15:04:05")
	if _, err := fmt.Fprintf(f, "\n# Added on %s\n%s\n", ts, rule); err != nil {
		return false, err
	}
	return false, reloadPostfix()
}

// unblockSender removes email's rule (and its preceding "# Added on ..."
// comment/blank line, if present) from header_checks. Returns found=false
// if no matching rule existed.
func unblockSender(email string) (found bool, err error) {
	if err := requireRoot(); err != nil {
		return false, err
	}

	data, err := os.ReadFile(headerChecksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	removed := false
	for _, line := range lines {
		if m := blockRuleRe.FindStringSubmatch(line); m != nil && m[2] == email {
			removed = true
			if len(out) > 0 && addedCommentRe.MatchString(out[len(out)-1]) {
				out = out[:len(out)-1]
				if len(out) > 0 && out[len(out)-1] == "" {
					out = out[:len(out)-1]
				}
			}
			continue
		}
		out = append(out, line)
	}
	if !removed {
		return false, nil
	}
	if err := os.WriteFile(headerChecksPath, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		return false, err
	}
	return true, reloadPostfix()
}
