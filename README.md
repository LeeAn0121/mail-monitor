# mail-monitor

Postfix / Dovecot `mail.log`을 실시간으로 보여주는 TUI 대시보드 (Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea)).

## 설치 (Ubuntu/Debian)

```bash
curl -fsSL https://jggen0121.github.io/mail-monitor/install.sh | bash
```

수동으로 `.deb`만 받으려면 [Releases](https://github.com/jggen0121/mail-monitor/releases)에서
`mail-monitor_<version>_linux_<arch>.deb`를 받아 `sudo dpkg -i`로 설치.

## 실행

```bash
mail-monitor
```

`/var/log/mail.log` 읽기 권한 필요. 없으면 내부적으로 `sudo tail -F`를 시도한다.
비밀번호 프롬프트 없이 쓰려면:

```bash
echo "$USER ALL=(ALL) NOPASSWD: /usr/bin/tail" | sudo tee /etc/sudoers.d/mail-monitor
```

## 키보드 단축키

| 키 | 기능 |
|----|------|
| `f` | 사용자/도메인 필터 입력 |
| `1`~`5` | 이벤트 종류 토글 (로그인/수신/송신/반송/거절) |
| `space` | 일시정지 / 재개 |
| `c` | 화면 클리어 |
| `q` | 종료 |

## 개발

```bash
go build -o mail-monitor .
```

## 릴리스

`v*` 태그를 푸시하면 GitHub Actions가 [goreleaser](https://goreleaser.com/)로
`linux/amd64`, `linux/arm64` 바이너리 + `.deb` 패키지를 빌드해 GitHub Releases에 올린다.

```bash
git tag v0.1.0
git push origin v0.1.0
```
