# mail-monitor

Postfix / Dovecot `mail.log`을 실시간으로 보여주는 TUI 대시보드 (Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea)).

## 설치 / 업데이트

```bash
curl -fsSL https://leean0121.github.io/mail-monitor/install.sh | bash
```

이미 설치된 상태에서 다시 실행하면 최신 릴리스로 덮어 설치된다. 수동으로 `.deb`만 받으려면
[Releases](https://github.com/LeeAn0121/mail-monitor/releases)에서
`mail-monitor_<version>_linux_<arch>.deb`를 받아 `sudo dpkg -i`로 설치.

설치된 버전 확인:

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

실행하면 최근 100줄을 이력으로 먼저 보여준 다음 실시간 tail로 이어진다 (전체 로그 리플레이
안 함 — 하루치를 다 불러오면 로그인 이벤트만으로도 화면이 뒤덮인다). 이후 들어오는 이벤트는
최근 5000건까지 버퍼에 보관되고, `f`로 검색하면 그 버퍼 전체가 대상 — 로그 원문뿐 아니라
디코딩된 제목/조회된 이름까지 검색된다.

LOGIN은 기본적으로 목록에서 숨겨져 있다 (세션마다 로그인/로그아웃이 찍혀서 가장 시끄러움).
카운트는 계속 집계되며, `1`로 토글하면 보인다.

별칭/포워딩 규칙으로 다른 주소에 배달된 메일이면(postfix의 `orig_to=`) 수신 옆에
원래 주소도 같이 표시된다: `수신: yhsohn44@outlook.com (yhsohn@koolsign.net에서 전달됨)`.

긴 줄은 터미널 폭에 맞춰 `…`로 잘려서 표시되고, 제목([Subject])은 항상 별도 줄로
붙는다. SRS로 재작성된 발신 주소(`SRS0=...`)는 원래 local@domain으로 축약해서 보여준다.
같은 발신자·같은 제목·같은 시각으로 여러 로컬 메일함에 동시에 배달된 메일(뉴스레터,
별칭 팬아웃 등)은 `수신: 3명 (hslee, jmpark, skpark)`처럼 한 줄로 묶이고, 포워딩 안내는
전부 같은 경우 제목 줄에 한 번만 붙는다. `users` 테이블 조회가 켜져 있고 이름이 있으면
local part 대신 이름으로 보여준다 (`수신: 3명 (이승희, 박정민, 박수경)`), 없으면 지금처럼
local part만.

## 이력 검색

`f`는 지금 메모리 버퍼(최근 5000건)만 검색한다. 그보다 오래된 것 — 로테이션된
`mail.log.1`, `mail.log.2.gz` 같은 파일까지 뒤지려면 `/`. 디스크에서 직접 스캔해서
(gzip도 풀어서) Queue-ID로 발신/수신 상관관계 다시 매칭한 다음 검색어와 맞는 것만 보여준다.
`mail.log`를 직접 읽을 권한이 없으면 `sudo cat`/`sudo zcat`로 자동 대체한다 (라이브
tail이 이미 하던 것과 동일한 fallback). `esc`로 실시간 화면으로 복귀.

## 발신량 랭킹

`r`을 누르면 지금 버퍼에 있는 SENT 이벤트를 발신자 주소별로 세서 [ntcharts](https://github.com/NimbleMarkets/ntcharts)
가로 막대그래프로 보여준다. 1위 발신자는 빨강, 순위가 낮아질수록 점점 SENT 색(주황)에
가까워지는 그라디언트라 튀는 발신자가 눈에 바로 들어온다. 평소보다 훨씬 많이 보내는
계정이 있으면 탈취돼서 스팸 발송에 쓰이고 있는 걸 수도 있다. `esc`로 복귀.

## 트래픽 스파크라인

헤더의 `TRAFFIC` 줄은 초당 전체 이벤트 수(로그인+수신+발신+반송+거절 합계)를
[ntcharts](https://github.com/NimbleMarkets/ntcharts) 스파크라인으로 보여준다.
브로드캐스트가 몰리거나 로그인 시도가 몰리는 순간이 로그를 스크롤하기 전에
그래프로 먼저 보인다.

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

**3. 반영** (재시작 불필요, 활성 연결 안 끊김)

```bash
sudo postfix reload
```

적용되면 로그에 이런 줄이 남고, mail-monitor가 Queue-ID로 상관관계 매칭해서 이벤트 끝에
`[제목]`으로 붙여준다:

```
postfix/cleanup[pid]: QUEUEID: warning: header Subject: 견적서 요청드립니다 from host[ip]; from=<...> to=<...> ...
```

## 이름 표시 (선택, MySQL 조회)

`users` 테이블(`email`, `name` 컬럼)에서 실명을 조회해 `이름 <email>` 형태로 보여줄 수 있다.
DB 접속정보는 코드에 넣지 않고 환경변수 `MAIL_MONITOR_DB_DSN`으로 전달한다:

```bash
export MAIL_MONITOR_DB_DSN="user:password@tcp(127.0.0.1:3306)/dbname?timeout=2s"
mail-monitor
```

매번 export 하기 귀찮으면 `.env` 파일로 관리 가능. 다음 순서로 찾아서 읽는다 (먼저 찾은 것만 사용):

1. `/etc/mail-monitor/.env` — 시스템 전역 (systemd/상시 실행 추천)
2. `./.env` — 실행 디렉토리 기준 (로컬 테스트용)

```bash
sudo mkdir -p /etc/mail-monitor
echo 'MAIL_MONITOR_DB_DSN=user:password@tcp(127.0.0.1:3306)/dbname?timeout=2s' | sudo tee /etc/mail-monitor/.env
sudo chmod 600 /etc/mail-monitor/.env
```

이미 환경변수가 설정돼 있으면 `.env` 값은 무시된다(환경변수 우선). `.env`는 절대 커밋하지
말 것 — `.gitignore`에 이미 등록돼 있음.

환경변수/`.env` 둘 다 없으면 조회 기능 자체가 비활성화되고(연결 시도 안 함) 이메일 원문
그대로 표시된다. DB가 죽어있거나 DSN이 틀려도 앱은 정상 기동하고 조회만 건너뛴다 — 시작 시
최대 2초 연결 시도 지연만 있음.

## 키보드 단축키

| 키 | 기능 |
|----|------|
| `f` | 필터 (메모리 버퍼, 최근 5000건 대상) |
| `/` | 이력 검색 (디스크의 mail.log + 로테이션 로그 대상, `esc`로 복귀) |
| `r` | 발신량 랭킹 — SENT를 발신자별로 집계해서 상위 순 표시 (`esc`로 복귀) |
| `1`~`5` | 이벤트 종류 토글 (로그인/수신/송신/반송/거절) |
| `space` | 일시정지 / 재개 |
| `c` | 화면 클리어 |
| `q` | 종료 |

## ops/

실제 운영 서버(postfix header_checks, spamassassin local.cf 추가분, unbound 리졸버 설정,
격리 메일함 정리용 cron 스크립트)에 적용한 설정 사본을 기록용으로 보관한다. 자동 배포되는
게 아니라 그냥 참고용 — 서버에 직접 적용해야 한다.

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
