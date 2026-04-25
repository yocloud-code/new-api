package minimax

// https://www.minimaxi.com/document/guides/chat-model/V2?id=65e0736ab2845de20908e2dd

var ModelList = []string{
	// Chat models
	"MiniMax-M2.7",
	"MiniMax-M2.7-highspeed",
	"MiniMax-M2.5",
	"MiniMax-M2.5-highspeed",
	"M2-her",
	"MiniMax-M2.1",
	"MiniMax-M2.1-highspeed",
	"MiniMax-M2",
	"abab6.5-chat",
	"abab6.5s-chat",
	"abab6-chat",
	"abab5.5-chat",
	"abab5.5s-chat",
	// TTS models
	"speech-2.8-hd",
	"speech-2.8-turbo",
	"speech-2.6-hd",
	"speech-2.6-turbo",
	"speech-2.5-hd-preview",
	"speech-2.5-turbo-preview",
	"speech-02-hd",
	"speech-02-turbo",
	"speech-01-hd",
	"speech-01-turbo",
	// Image models
	"image-01",
	"image-01-live",
	// Music models
	"music-2.5",
}

var ChannelName = "minimax"
