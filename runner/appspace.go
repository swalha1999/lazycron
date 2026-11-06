package runner

import (
	"context"
	"os/exec"
	"time"
)

// NameAppSpace asks a herdr-app window running on this machine to call the
// space spaceID name. herdr-app has no label for space names (it names a
// space after its first terminal's folder), so this makes the name stick
// when the app runs here. It does nothing when herdr-app is not installed or
// its window is not running; `herdr-app status` only talks to a running
// window and never starts the app.
func NameAppSpace(spaceID, name string) {
	app, err := exec.LookPath("herdr-app")
	if err != nil || spaceID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if exec.CommandContext(ctx, app, "status").Run() != nil {
		return
	}
	_ = exec.CommandContext(ctx, app, "do", "space.rename", "--id", spaceID, "--name", name).Run()
}
