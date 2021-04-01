package main

import (
	"embed"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/itchyny/volume-go"
	"github.com/pkg/browser"
)

//go:embed media/icon_mono.png
var fEmbed embed.FS

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	SetupCloseHandler()

	systray.Run(onReady, onExit)
}

func SetupCloseHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		systray.Quit()
	}()
}

func onReady() {
	// System Tray top-level
	dat, err := fEmbed.ReadFile("media/icon_mono.png")
	check(err)
	systray.SetTemplateIcon(dat, dat)
	systray.SetTooltip("Quiet Audio Fade")

	// Status Control
	mStatus := systray.AddMenuItemCheckbox("ACTIVE [-]", "Toggle to enable/disable audio fading", true)

	// Read the volume at a regular interval to update the UI
	checkVolumeTicker := time.NewTicker(5 * time.Second)
	volumeReading := make(chan int)
	checkVolumeDone := make(chan bool)
	go func() {
		previousVolume, err := volume.GetVolume()
		check(err)
		for {
			select {
			case <-checkVolumeDone:
				return
			case <-checkVolumeTicker.C:
				vol, err := volume.GetVolume()
				check(err)
				volumeReading <- vol

				// Check how much the volume has changed
				volDiff := previousVolume - vol
				if !(volDiff == 0 || volDiff == 1 || volDiff == 2) {
					// If the volume changed too much that means the user changed it
					// Pause the volume changing if it is active
					if mStatus.Checked() {
						mStatus.ClickedCh <- struct{}{}
					}
				}
				previousVolume = vol
			}
		}
	}()

	// Status button functionality
	go func() {
		mostRecentVolume := 0
		for {
			select {
			case mostRecentVolume = <-volumeReading:
				// Update volume when there is a new volume reading
				if mStatus.Checked() {
					mStatus.SetTitle("ACTIVE  [" + strconv.Itoa(mostRecentVolume) + "]")
				} else {
					mStatus.SetTitle("PAUSED  [" + strconv.Itoa(mostRecentVolume) + "]")
				}
			case <-mStatus.ClickedCh:
				// Enable or diable when clicked
				if mStatus.Checked() {
					mStatus.Uncheck()
					mStatus.SetTitle("PAUSED  [" + strconv.Itoa(mostRecentVolume) + "]")
				} else {
					mStatus.Check()
					mStatus.SetTitle("ACTIVE  [" + strconv.Itoa(mostRecentVolume) + "]")
				}
			}
		}
	}()

	// Get an initial volume reading
	vol, err := volume.GetVolume()
	check(err)
	volumeReading <- vol

	systray.AddSeparator()

	// Speed Options
	currentFadeSpeed := make(chan time.Duration)
	fadeSpeeds := [3]time.Duration{7, 3, 1}
	fadeSpeedNames := [3]string{"7min", "3min", "1min"}
	fadeSpeedIndex := 1

	// Volume Setter
	changeVolumeTicker := time.NewTicker(fadeSpeeds[fadeSpeedIndex] * time.Minute)
	changeVolumeDone := make(chan bool)
	go func() {
		var lastVolume int
		for {
			select {
			case <-changeVolumeDone:
				return
			case lastVolume = <-volumeReading:
			case speed := <-currentFadeSpeed:
				// Reset the timer if the speed changes
				changeVolumeTicker.Reset(speed * time.Minute)
			case <-changeVolumeTicker.C:
				// If active, change the volume at a regular interval
				if mStatus.Checked() {
					// Don't set negative volumes
					if lastVolume >= 1 {
						setErr := volume.SetVolume(lastVolume - 2)
						check(setErr)
						volumeReading <- lastVolume - 2
					} else {
						mStatus.ClickedCh <- struct{}{}

					}
				}
			}
		}
	}()

	// Speed Change
	mSpeed := systray.AddMenuItem("Speed  ["+fadeSpeedNames[fadeSpeedIndex]+"]", "Fade speed - click to change")
	go func() {
		for {
			<-mSpeed.ClickedCh
			fadeSpeedIndex += 1
			if fadeSpeedIndex >= len(fadeSpeeds) {
				fadeSpeedIndex = 0
			}
			mSpeed.SetTitle("Speed  [" + fadeSpeedNames[fadeSpeedIndex] + "]")
			currentFadeSpeed <- fadeSpeeds[fadeSpeedIndex]
		}
	}()

	systray.AddSeparator()

	// About Website
	mAbout := systray.AddMenuItem("About", "Go to this project's website")
	go func() {
		for {
			<-mAbout.ClickedCh
			err := browser.OpenURL("https://github.com/StuffJackMakes/Quiet-Audio-Fade")
			check(err)
		}
	}()

	// Quit Application
	mQuit := systray.AddMenuItem("Quit", "Exit Quiet Audio Fade")
	go func() {
		<-mQuit.ClickedCh
		checkVolumeDone <- true
		changeVolumeDone <- true
		systray.Quit()
	}()
}

func onExit() {
	os.Exit(0)
}
