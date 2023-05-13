package expose

const(
  // EndOfResponseMarker edgy has to signal the end of response
  //TODO: implement a binary transfer and send the length of the response upfront, so the server will know how much to read
  EndOfResponseMarker = "<!--xmark:end-->"
  // NoExplicitHostRequest the client doesn't have a desired hostname, so the cloud will generate one
  NoExplicitHostRequest = "none"

)
