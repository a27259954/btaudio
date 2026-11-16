## What changes

<!-- The user-visible change, and why. -->

## How it was tested

<!-- Tests added or run, and manual checks with a real phone if it touches Bluetooth (phone, OS and Windows version). -->

## Checklist

- [ ] `go vet ./...`, `go test ./internal/...` and `npm --prefix frontend run check` pass
- [ ] New UI strings are in every language in `frontend/src/i18n.js` (tray strings in `internal/lang`)
- [ ] Behaviour changes in `internal/core` come with a test against the fake backend
- [ ] README (English and 繁體中文) updated if behaviour changed
- [ ] No secrets, personal data or local AI instruction files are included
