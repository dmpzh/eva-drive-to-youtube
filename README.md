# eva-cli

Small personal CLI for listing EVA VR replay videos from Google Drive, downloading selected sessions, optionally merging them with FFmpeg, and uploading the result to YouTube as an unlisted video.

## Requirements

- Go 1.24+
- FFmpeg and FFprobe available in PATH
- A Google Cloud OAuth client for a desktop application, see [Google Cloud setup](#google-cloud-setup)
- Google Drive API enabled
- YouTube Data API v3 enabled

## Configuration

Create `config.yaml` in the project root:

```yaml
drive_folder_id: "FOLDER_ID"
download_dir: "./tmp"
credentials_file: "./credentials.json"
token_file: "./token.json"
```

Place your OAuth desktop application credentials JSON at `credentials.json`.

If you have not created it yet, follow [Google Cloud setup](#google-cloud-setup).

Keep these files local only:

- `credentials.json`
- `token.json`
- `config.yaml`

They are ignored by git and should not be committed.

## Commands

Authenticate once:

```bash
eva-cli login
```

List sessions for a date:

```bash
eva-cli list --date 2026-03-10
eva-cli list --date yesterday
```

Upload one or more sessions:

```bash
eva-cli upload --date yesterday
eva-cli upload --date 2026-03-10
eva-cli upload --date yesterday --sessions "1 2"
eva-cli upload --date yesterday --ignore-corrupt
```

Without `--sessions`, the CLI opens an interactive multi-select prompt showing the available sessions. Use the arrow keys to move, space to toggle a session, and enter to confirm the upload.

When multiple sessions are selected, the tool generates an FFmpeg concat list and merges the videos without re-encoding when possible.

Before uploading a merged video, the CLI validates each downloaded MP4 and checks that the merged duration matches the sum of the selected sessions.

If you pass `--ignore-corrupt`, sessions that are invalid are skipped. If no valid session remains, the upload is canceled.

Before uploading, the CLI asks for the title suffix and pre-fills `Session Review`. You can press enter to keep it, or replace it with something else such as `Training Review`.

Before any download starts, the CLI shows a final summary with the selected sessions, the YouTube title, and the privacy status, then asks for confirmation.

## Quick start for login

1. Copy `config.example.yaml` to `config.yaml`.
2. Replace `FOLDER_ID` with your Google Drive shared folder ID.
3. Follow [Google Cloud setup](#google-cloud-setup), then place the downloaded desktop OAuth JSON file at `credentials.json`.
4. Build the binary:

```bash
go build -o eva-cli.exe .
```

5. Run the login flow:

```bash
./eva-cli.exe login
```

After login succeeds, you can verify access with:

```bash
./eva-cli.exe list --date yesterday
```

## Google Cloud setup

This project uses a personal OAuth flow with a desktop application client. You need your own Google Cloud project before `eva-cli login` can work.

### 1. Create or select a Google Cloud project

Open the Google Cloud Console and create a new project, or reuse an existing one dedicated to this tool.

### 2. Enable the required APIs

In `APIs & Services > Library`, enable:

- Google Drive API
- YouTube Data API v3

### 3. Configure the OAuth consent screen

In `APIs & Services > OAuth consent screen`:

1. Choose `External` for a personal Google account.
2. Fill in the minimum required fields:
	- App name
	- User support email
	- Developer contact email
3. Save the form.

For personal use, keep the app in `Testing` mode.

### 4. Add yourself as a test user

If the app is not verified by Google, only test users can log in.

In the same OAuth consent screen page, add the Google account you will use for Drive and YouTube under `Test users`.

### 5. Create desktop OAuth credentials

In `APIs & Services > Credentials`:

1. Click `Create credentials`.
2. Choose `OAuth client ID`.
3. Select `Desktop app`.
4. Create the client.
5. Download the generated JSON file.

Rename that file to `credentials.json` and place it in the project root.

Important:

- Do not create a service account. YouTube uploads to a personal channel should use OAuth user credentials.
- If you regenerate the OAuth client, replace `credentials.json` and run `eva-cli login` again.

### 6. What happens during login

When you run `eva-cli login`, the CLI:

- opens your browser,
- asks you to approve Drive read access and YouTube upload access,
- receives the OAuth callback on `localhost`,
- stores the resulting token in `token.json`.

If Google shows `This app is not verified`, continue only if you trust your own local app and you are using a test user that you added yourself.