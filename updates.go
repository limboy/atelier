package main

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
)

// checking is set while an update check or install runs, so that a second
// one does not start over it.
var checking atomic.Bool

// checkForUpdates looks for a newer release on GitHub and offers to install
// it. A check at launch (manual false) stays quiet unless there is an update;
// one the user asked for says so when the app is up to date or the check
// failed. It does nothing in development builds, which have no updates.
func (a *app) checkForUpdates(manual bool) {
	if !mygo.Updater.Enabled() || !checking.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer checking.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		up, err := mygo.Updater.Check(ctx)
		cancel()
		switch {
		case err != nil:
			if manual {
				mygo.Dialog.Message(mygo.MessageOptions{Parent: a.win, Type: mygo.MessageWarning,
					Message: "Couldn’t check for updates.", Detail: err.Error()})
			}
			return
		case up == nil:
			if manual {
				mygo.Dialog.Message(mygo.MessageOptions{Parent: a.win,
					Message: "Atelier is up to date.", Detail: "Version " + mygo.App.Version() + " is the latest."})
			}
			return
		}
		res, _ := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  a.win,
			Message: "Atelier " + up.Version + " is available.",
			Detail:  up.Notes,
			Buttons: []string{"Install and Restart", "Later"},
		})
		if res.Button != 0 {
			return
		}
		// Downloading a release takes a while, so installing has no time limit.
		if err := up.Install(context.Background(), nil); err != nil {
			mygo.Dialog.Message(mygo.MessageOptions{Parent: a.win, Type: mygo.MessageWarning,
				Message: "Couldn’t install the update.", Detail: err.Error()})
			return
		}
		mygo.App.Relaunch()
	}()
}
