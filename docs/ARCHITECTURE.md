# Architecture

One nonblocking `AF_PACKET/SOCK_RAW` socket, bound to `ETH_P_ARP`, is created per configured interface. A single epoll loop drains ready sockets. An eventfd serializes interface-set changes and shutdown into that loop. Frames are parsed and replies are built in reusable stack buffers. The target set is represented by sorted, merged IPv4 integer intervals and queried by binary search.

Configuration reload is transactional at the parser level: invalid YAML or targets retain the last working snapshot. New interface sockets are all opened before obsolete sockets are removed. fsnotify watches parent directories (which also detects atomic file replacement), while a periodic scan provides a fallback.
