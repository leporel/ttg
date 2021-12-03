# ttg
Pass users permissions from twitch to telegram chat

### How it works

Self-hosted application  
After you pass parameters to execute the program, bot will restrict send messages in telegram chat for new users  
User must get and open link and approve that he has follow to channel

### Features

* Ability to add user to white list 
* All exising users in chat was ignored  
* The bot checks (every 30 minutes) for channel followers and update permissions to registered by bot users

### How to build 

* Install [golang 1.17.3](https://golang.org/dl/)
* If you on Windows, install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/download/)
* In project folder run console command  
 `go mod tidy`  
 `go build .`

### Execute 

`.\ttg.exe -help`  
`.\ttg.exe -app **** -code **** -channel leporel -group -100137328159 -host localhost -owner 7007777 -token ****`  

### Notes

You need register [twitch app](https://dev.twitch.tv/console/apps) and create [telegram bot](https://t.me/BotFather), add bot to group and give him admin rights  
TelegramID and groupID you can get via this [bot](https://t.me/myidbot) 
TwitchID you cat get here

Not tested in real world, tested on local machine ```host = localhost ```  
Probably, if twitch required use https, idk, you need reverse proxy to set up https callback on twitch apps, or improve code to use certs  
I assume that the application can be uploaded to heroku apps