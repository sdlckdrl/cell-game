# Cell Game Mobile

이 폴더는 현재 `D:\project\game` 웹 게임을 감싸는 Android WebView 앱입니다.

## 구성

- `app-url.properties`: 앱이 접속할 웹 서버 주소
- `app/`: Android 앱 코드
- `gradlew.bat`: Windows용 Gradle Wrapper

## 서버 주소 설정

`app-url.properties`에서 빌드별 접속 주소를 바꿉니다.

- 에뮬레이터 디버그: 기본값 `http://10.0.2.2:8000`
- 실기기 디버그: PC의 로컬 IP로 변경
- 릴리스 배포: 반드시 HTTPS 주소로 변경

예시:

```properties
debugBaseUrl=http://192.168.0.20:8000
releaseBaseUrl=https://game.example.com
```

## 실행

먼저 루트 프로젝트에서 Go 서버를 실행합니다.

```powershell
go run .
```

그다음 모바일 프로젝트에서 디버그 APK를 빌드합니다.

```powershell
cd D:\project\game\mobile
$env:JAVA_HOME='C:\Program Files\Android\Android Studio\jbr'
$env:Path='C:\Program Files\Android\Android Studio\jbr\bin;' + $env:Path
.\gradlew.bat assembleDebug
```

## 배포 전 체크

- `releaseBaseUrl`을 실제 HTTPS 서버로 변경
- `keystore.properties`와 `signing/cellgame-release.jks`를 안전한 곳에 백업
- Android Studio에서 앱 아이콘/서명키 점검
- `assembleRelease` 또는 번들 빌드 진행
