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

실행 시 오늘 하루치 이력을 먼저 보여준 뒤 실시간 tail로 이어진다. LOGIN은 기본적으로
숨겨져 있으며 `1`로 토글한다.

## 웹 대시보드

`.deb`로 설치하면 `mail-monitor.service`가 systemd에 등록되어 부팅 시 자동으로 뜨고,
`http://localhost:8080` 에서 실시간 대시보드를 볼 수 있다 (React + MUI, Pretendard 폰트).
헤드리스로 동작하므로(`mail-monitor --daemon`) 터미널을 안 열어도 항상 살아있다.

```bash
sudo systemctl status mail-monitor     # 서비스 상태
sudo systemctl restart mail-monitor    # 재시작
journalctl -u mail-monitor -f          # 로그
```

터미널에서 `mail-monitor`를 직접 실행하면 TUI가 뜬다. 서비스가 이미 8080을 쓰고 있으므로
TUI에서 웹서버를 다시 띄우고 싶지 않으면 `MAIL_MONITOR_WEB_ADDR=off`로 끄고 실행:

```bash
MAIL_MONITOR_WEB_ADDR=off mail-monitor        # TUI만, 웹서버 끄기
MAIL_MONITOR_WEB_ADDR=:9090 mail-monitor      # 포트 변경
```

프론트엔드는 Go 바이너리에 임베드되어 있어 별도 배포 없이 단일 바이너리로 동작한다.
소스는 `web/`에 있으며, 수정 후 `cd web && npm run build`로 다시 빌드해야 바이너리에 반영된다.

## 이력 검색

`f`는 지금 메모리 버퍼(최근 5000건)만 검색한다. 그보다 오래된 것 — 로테이션된
`mail.log.1`, `mail.log.2.gz` 같은 파일까지 뒤지려면 `/`. 디스크에서 직접 스캔해서
(gzip도 풀어서) Queue-ID로 발신/수신 상관관계 다시 매칭한 다음 검색어와 맞는 것만 보여준다.
`mail.log`를 직접 읽을 권한이 없으면 `sudo cat`/`sudo zcat`로 자동 대체한다 (라이브
tail이 이미 하던 것과 동일한 fallback). `esc`로 실시간 화면으로 복귀.

## 발신/수신량 랭킹

`r`을 누르면 지금 버퍼에 있는 SENT 이벤트를 발신자 주소별로, `R`을 누르면 RECV/FWD
이벤트를 수신자 주소별로 세서 [ntcharts](https://github.com/NimbleMarkets/ntcharts)
가로 막대그래프로 보여준다. 1위는 빨강, 순위가 낮아질수록 점점 원래 색(발신=주황,
수신=하늘색)에 가까워지는 그라디언트라 튀는 주소가 눈에 바로 들어온다. 평소보다 훨씬
많이 보내는 계정이 있으면 탈취돼서 스팸 발송에 쓰이고 있는 걸 수도 있고, 특정
사서함이 비정상적으로 많이 받고 있으면 포워딩 alias 오남용이나 표적 스팸일 수 있다.
`esc`로 복귀.

## 전달(forwarding) 표시

postfix가 `orig_to=`로 최종 수신함과 다른 주소로 배달했다고 로그를 남기면 — alias나
포워딩 규칙이 적용된 경우 — 해당 이벤트는 RECV 대신 별도의 FWD 타입으로 표시된다.
`6`으로 켜고 끌 수 있고, 목록에서는 `↪` 글리프로 구분된다.

## BOUNCE/REJECT 급증 경고

1초 사이 BOUNCE+REJECT가 3건 이상 늘면 상태바에 빨간 `⚠ BOUNCE/REJECT 급증` 배지가
10초간 뜬다. 스팸 발송 시도나 릴레이 설정 오류로 인한 순간적인 반송/거절 폭주를
로그를 눈으로 훑기 전에 알아챌 수 있다.

## CSV 내보내기

`e`를 누르면 현재 화면에 보이는 이벤트(타입 토글 + 필터 적용된, 이력 검색 중이면
검색 결과)를 `mail-monitor-<타임스탬프>.csv`로 실행 디렉토리에 저장한다.

## 트래픽 스파크라인

헤더의 `TRAFFIC` 줄은 초당 전체 이벤트 수(로그인+수신+발신+전달+반송+거절 합계)를
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
| `R` | 수신량 랭킹 — RECV/FWD를 수신자별로 집계해서 상위 순 표시 (`esc`로 복귀) |
| `e` | 현재 화면의 이벤트를 CSV로 내보내기 |
| `1`~`6` | 이벤트 종류 토글 (로그인/수신/송신/전달/반송/거절) |
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
