#!/bin/bash
# /usr/local/sbin/sa-learn-quarantine.sh
#
# spam-quarantine@koolsign.net Maildir에 쌓인 메일을 SpamAssassin Bayes에
# 학습시키고 지운다. header_checks의 REDIRECT 규칙으로 스팸 판정된 메일이
# 여기로 모임 (ops/postfix/header_checks 참고).
#
# crontab: 0 3 * * * /usr/local/sbin/sa-learn-quarantine.sh

set -e
MAILDIR=/data/vmail/koolsign.net/spam-quarantine
LOG=/var/log/sa-learn-quarantine.log

files=$(find "$MAILDIR/cur" "$MAILDIR/new" -type f 2>/dev/null)
[ -z "$files" ] && exit 0

echo "$files" | xargs sa-learn --spam --username=spamd >> "$LOG" 2>&1
echo "$files" | xargs rm -f
