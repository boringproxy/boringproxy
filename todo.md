* Implement auth
* I don't think it's properly closing connections. Browser are hanging on
  some requests, possibly because it's HTTP/1.1 and hitting the max concurrent
  requests.
* Might want to proxy requests at the HTTP level since it lets us do things
  like terminating HTTP/2.
