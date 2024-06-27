# Cron job scheduling

Cron is a task scheduler that allows user to automate commands or scripts to 
run at specific times, dates, or intervals. This makes cron a powerful tool for system administrators and developers who 
want to automate repetitive tasks.

What can users automate with cron?

- **System maintenance**: Cron can be used to schedule regular backups, update software packages, or clean up temporary files.
- **Data processing**: Users can use cron to download data from the internet at specific times, process it, and generate reports.
- **Sending notifications**: Cron can be used to trigger emails or other notifications based on events or system logs.

Basically, any task that can be expressed as a command or script can be automated with cron.

Writing a cron job!
On Linux like systems cron jobs can be added by adding a line to the crontab file, specifying the schedule and the command
that needs to be run at that schedule. The cron schedule is expressed in the following format.

`minute hour day_of_month month day_of_week`

Each field can take a specific value or combination of values to define the schedule. Users can use special characters like 
`*` (asterisk) to represent **any** value and `,` (comma) to separate multiple values. It also supports `0-n` to define a
range of values for which the cron should run and `*/n` to define number of times the cron should run. Here n is an integer.

## Adding cron jobs in GoFr applications
Adding cron jobs to GoFr applications is made easy with a simple injection of user's function to the cron table maintained
by the GoFr. The minimum time difference between cron job's two consecutive runs is a minute as it is the least significant
scheduling time parameter.
```go
app.AddCronJob("* * * * *", "job-name", func(ctx *gofr.Context) {
	// the cron job that needs to be executed at every minute
})
```
The `AddCronJob` methods takes three arguments—a cron schedule, the cron job name(for tracing) and the set of statements 
that are to be executed at the given schedule.

### Example

```go
package main

import (
	"time"
	
	"gofr.dev/pkg/gofr"
)

func main() {
    app := gofr.New()

	// Run the cron job every 5 hours(*/5)
	app.AddCronJob("* */5 * * *", "", func(ctx *gofr.Context) {
		ctx.Logger.Infof("current time is %v", time.Now())
	})

	// 
	app.Run()
}
```