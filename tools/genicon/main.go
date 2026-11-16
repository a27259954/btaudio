// Writes build/appicon.png and frontend/public/icon.png. Run from the project root:
//
//	go run ./tools/genicon
package main

import (
	"os"

	"github.com/a27259954/btaudio/internal/icon"
)

func main() {
	must(os.WriteFile("build/appicon.png", icon.Render(1024, icon.Brand), 0o644))
	must(os.WriteFile("frontend/public/icon.png", icon.Render(128, icon.Brand), 0o644))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
