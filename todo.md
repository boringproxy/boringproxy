* I don't think it's properly closing connections. Browser are hanging on
  some requests, possibly because it's HTTP/1.1 and hitting the max concurrent
  requests.
* Might want to proxy requests at the HTTP level since it lets us do things
  like terminating HTTP/2.
* Implement a custom SSH server in Go and connect the sockets directly?
* Use HTML redirects for showing errors then refreshing. Maybe for polling 
  after login and submitting a new tunnel too.
