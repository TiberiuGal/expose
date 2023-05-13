# expose
inspired by ngrok

Allows you to expose any local webserver to the web

# Project structure
There are 2 main components 
- [edgy] sits on the local machine , it connects to cloudy and proxies requests comming from cloudy to the specified local server  
- [cloudy] sits in the cloud and proxies incomming requests to a specific edgy , based on hostname

