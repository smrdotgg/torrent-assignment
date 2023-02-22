# DSP
## Usage
To use the torrent client, simply go to the root directory of the folder. An already compiled executable can be found, named ‘client.bin’. If your machine does not have the golang compiler installed, you can use this executable. If you would like to compile the code yourself, open your terminal and type 
```
$ go build .
```

Then, run the client as follows. Type in `./client.bin <torrent_file_dir>`. The torrent will connect to the tracker server, get the peer list, and start sharing files.


## Technical Description
What happens in this program can be broken down into two phases. 
1. Preprocessing
2. Fetching

### Preprocessing
In this stage, the program takes in the torrent file, translates the bencoded format into a generic Javascript object (JSON), and then encodes it as a golang struct. This is what happens in the first half of the core main loop, and takes less than a second to finish on most devices.
### Fetching
This is where the crux of the program lies. Once the program attains the client list, a download worker is deployed for each peer in the list. A download worker is simply a thread that waits for items to download in the work queue channel, and downloads it. This is where all of the three core elements required by the documentation come to play.
1. Go-Routines: download workers are deployed as goroutines. (line 153 in torrentFile/torrentFile.go)
2. Channels:  Each download worker looks to the “Work Queue” for bits to download. The work queue in actuality is just a channel where bits of torrent slices to download are placed. As you can see, the work queue channel is given as a parameter for the download worker function call. (lines 87 and 100 in torrentFile/torrentFile.go)
3. Sockets:  Each connection with a peer uses sockets. These sockets are used to communicate with each other by sending signals like choke on unchoke. As can be seen in the following, the message {ID:Unchoke} is being serialized and sent. (line 158 in message/message.go)



