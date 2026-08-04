# mail-monitor

Postfix / Dovecot `mail.log`을 실시간으로 보여주는 TUI 대시보드 (Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea)).

## 설치 (Ubuntu/Debian)

```bash
curl -fsSL https://LeeAn0121.github.io/mail-monitor/install.sh | bash
```

수동으로 `.deb`만 받으려면 [Releases](https://github.com/LeeAn0121/mail-monitor/releases)에서
`mail-monitor_<version>_linux_<arch>.deb`를 받아 `sudo dpkg -i`로 설치.

## 업데이트

설치 스크립트를 다시 실행하면 최신 릴리스로 덮어 설치된다:

```bash
curl -fsSL https://leean0121.github.io/mail-monitor/install.sh | bash
```

현재 설치된 버전 확인:

```bash
mail-monitor --version
```

## 실행

```bash
mail-monitor
```

`/var/log/mail.log` 읽기 권한 필요. 없으면 내부적으로 `sudo tail -F`를 시도한다.
비밀번호 프롬프트 없이 쓰려면:

```bash
echo "$USER ALL=(ALL) NOPASSWD: /usr/bin/tail" | sudo tee /etc/sudoers.d/mail-monitor
```

## 메일 제목(Subject) 표시 (선택, 서버 설정 필요)

Postfix 기본 로그엔 제목이 안 남는다. `header_checks`로 Subject만 평문으로 syslog에 남기도록
설정하면 mail-monitor가 자동으로 집어서 보여준다.

⚠️ 제목이 syslog에 평문으로 저장됨 — 로그 접근 권한/보존 정책 고려하고 적용할 것.

**1. `/etc/postfix/header_checks` 생성**

```
/^Subject:/ WARN
```

**2. `/etc/postfix/main.cf`에 추가**

```
header_checks = regexp:/etc/postfix/header_checks
```

**3. 반영**

```bash
sudo postfix reload
```

`postfix reload`만으로 충분함 (재시작 불필요, 활성 연결 안 끊김).

적용되면 로그에 이런 줄이 남고, mail-monitor가 Queue-ID로 상관관계 매칭해서 이벤트 끝에
`[제목]`으로 붙여준다:

```
postfix/cleanup[pid]: QUEUEID: warning: header Subject: 견적서 요청드립니다 from host[ip]; from=<...> to=<...> ...
```

## 이름 표시 (선택, MySQL 조회)

`users` 테이블(`email`, `name` 컬럼)에서 실명을 조회해 `이름 <email>` 형태로 보여줄 수 있다.
DB 접속정보는 코드/설정파일에 넣지 않고 환경변수로 전달한다:

```bash
export MAIL_MONITOR_DB_DSN="user:password@tcp(127.0.0.1:3306)/dbname?timeout=2s"
mail-monitor
```

환경변수 없으면 조회 기능 자체가 비활성화되고(연결 시도 안 함) 이메일 원문 그대로 표시된다.
DB가 죽어있거나 DSN이 틀려도 앱은 정상 기동하고 그냥 조회를 건너뛴다 — 시작 시 최대 2초
연결 시도 지연만 있음.

systemd 서비스로 상시 실행한다면 `Environment=` 또는 `EnvironmentFile=`로 넣는 걸 추천
(쉘 히스토리/프로세스 목록에 비밀번호 안 남음).

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
