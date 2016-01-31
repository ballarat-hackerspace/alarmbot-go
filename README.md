# Alarmbot Go

The alarmbot re-written in `go`. It listens for UDP broadcasts from a Particle
Photon which has a PIR (and other sensors) to send alerts to our Slack. It
has a few hard coded bits in there for our Hackerspace but at the very least
it's useful as example code.

# Build and Run

```
$ go get github.com/ballarat-hackerspace/alarmbot-go
$ go build github.com/ballarat-hackerspace/alarmbot-go
$ echo "slack_api: your-slack-webhook-api-token-here" > config.yml
$ ./alarmbot-go
```
