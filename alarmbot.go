package main

import (
	"fmt"
	"github.com/nlopes/slack"
	"github.com/spf13/viper"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func chk(err error) {
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}
}

// return etime, channel, data
func Split(s string) (string, string, string) {
	if len(s) == 0 {
		return "", "", ""
	}

	slice := strings.SplitN(s, " ", 2)

	if len(slice) != 2 {
		return "", "", ""
	}

	slice2 := strings.SplitN(slice[1], "=", 2)

	if len(slice2) != 2 {
		return "", "", ""
	}

	return slice[0], slice2[0], slice2[1]
}

func logToSlack(api *slack.Client, my_name string, channel string, msg string, fields []slack.AttachmentField) {
	params := slack.NewPostMessageParameters()
	params.Username = my_name
	params.IconEmoji = ":page_with_curl:"
	attachment := slack.Attachment{}
	attachment.Color = "#ffaa00"
	attachment.Title = "Log Entry"
	attachment.Text = msg
        attachment.Fields = fields
	params.Attachments = []slack.Attachment{attachment}
        api.PostMessage(channel, "", params)
}

func statusToSlack(api *slack.Client, my_name string, channel string, msg string) {
	params := slack.NewPostMessageParameters()
	params.Username = my_name
	params.IconEmoji = ":exclamation:"
	attachment := slack.Attachment{}
	attachment.Color = "#ffaa00"
	attachment.Title = "Alarm Status Update"
	attachment.Text = msg
	params.Attachments = []slack.Attachment{attachment}
	api.PostMessage(channel, "", params)
}

func lightsToSlack(api *slack.Client, my_name string, channel string, image string, level int) {
	_, err := http.Get(image)
	chk(err)

	params := slack.NewPostMessageParameters()
	params.Username = my_name
	params.IconEmoji = ":bulb:"
	attachment := slack.Attachment{}
	attachment.Color = "#00ff00"
	attachment.Title = "Lights detected"
	attachment.Text = fmt.Sprintf("Light level detected: %d", level)
	attachment.ImageURL = image
	params.Attachments = []slack.Attachment{attachment}
	api.PostMessage(channel, "", params)
}

func motionToSlack(api *slack.Client, my_name string, channel string, image string, count int) {
	_, err := http.Get(image)
	chk(err)

	params := slack.NewPostMessageParameters()
	params.Username = my_name
	params.IconEmoji = ":rotating_light:"
	attachment := slack.Attachment{}
	attachment.Color = "#ff0000"
	attachment.Title = "Motion detected"
	attachment.Text = fmt.Sprintf("Motion events detected: %d", count)
	attachment.ImageURL = image
	params.Attachments = []slack.Attachment{attachment}
	api.PostMessage(channel, "", params)
}

type Light struct {
	level     int
	lights_on bool
}

func main() {
	var activity_seen int64
	var last_motion_alert int64 = 0

	lights := Light{level: 0, lights_on: false}

	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/alarmbot-go/")
	viper.AddConfigPath("$HOME/.alarmbot-go/")
	viper.AddConfigPath(".")
	viper.SetDefault("port", 25276)
	viper.SetDefault("debugging", false)
	viper.SetDefault("squelch", 120)
	viper.SetDefault("lights_trip", 1500)
	viper.SetDefault("to_channel", "#security")
	viper.SetDefault("log_channel", "#logging")
	viper.SetDefault("my_name", "AlarmBot GO")
	err := viper.ReadInConfig()
	chk(err)

	slack_api := slack.New(viper.GetString("slack_api"))
        fields := []slack.AttachmentField{
          slack.AttachmentField{"debugging", viper.GetString("debugging"), true},
          slack.AttachmentField{"squelch", viper.GetString("squelch"), true},
          slack.AttachmentField{"log_channel", viper.GetString("log_channel"), true},
          slack.AttachmentField{"to_channel", viper.GetString("to_channel"), true},
          slack.AttachmentField{"port", viper.GetString("port"), true},
          slack.AttachmentField{"lights_trip", viper.GetString("lights_trip"), true},
        }
	logToSlack(slack_api, viper.GetString("my_name"), viper.GetString("log_channel"), "I'm online and monitoring", fields)

	ServerAddr, err := net.ResolveUDPAddr("udp", ":"+viper.GetString("port"))
	chk(err)

	ServerConn, err := net.ListenUDP("udp", ServerAddr)
	defer ServerConn.Close()
	chk(err)

	buf := make([]byte, 1024)
	activity_seen = time.Now().Unix()

	watchdog := time.NewTicker(150 * time.Second)
	go func() {
		for t := range watchdog.C {
			last_activity := t.Unix() - activity_seen
			fmt.Println("activity last seen:", last_activity, "seconds ago")
			if last_activity > 600 {
				// TODO: reboot core via spark api (https://github.com/getniwa/particle)
				fmt.Println("Nothing seen from core in over 10 minutes, please reboot")
			}
		}
	}()

	for {
		n, _, err := ServerConn.ReadFromUDP(buf)
		chk(err)

		etime, channel, data := Split(string(buf[0:n]))
		fmt.Println("Time:", etime, "Channel:", channel, "Data:", data)
		// logToSlack(slack_api, viper.GetString("my_name"), viper.GetString("log_channel"), fmt.Sprintf("Channel: %s, Data: %s", channel, data))

		activity_seen = time.Now().Unix()

		if channel == "ballarathackerspace.org.au/status" {
			statusToSlack(slack_api, viper.GetString("my_name"), viper.GetString("to_channel"), data)
		} else if channel == "ballarathackerspace.org.au/motion" {
			count, err := strconv.Atoi(data)
			chk(err)
			if time.Now().Unix()-last_motion_alert > int64(viper.GetInt("squelch")) {
				motionToSlack(slack_api, viper.GetString("my_name"), viper.GetString("to_channel"), fmt.Sprintf("https://ballarathackerspace.org.au/webcam%s.jpg", etime), count)
			} else {
				fmt.Println("squelched a motion alarm")
			}
			last_motion_alert = time.Now().Unix()
		} else if channel == "ballarathackerspace.org.au/light" {
			lights.level, err = strconv.Atoi(data)
			chk(err)

			if lights.level > viper.GetInt("lights_trip") {
				if !lights.lights_on {
					lightsToSlack(slack_api, viper.GetString("my_name"), viper.GetString("to_channel"), fmt.Sprintf("https://ballarathackerspace.org.au/webcam%s.jpg", etime), lights.level)
					lights.lights_on = true
				}
			} else {
				lights.lights_on = false
			}
		} else if channel == "ballarathackerspace.org.au/watchdog" {
		} else if channel == "ballarathackerspace.org.au/wifi" {
		} else {
		}
	}
}
