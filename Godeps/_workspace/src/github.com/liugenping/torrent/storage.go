package torrent

import "os"

var fileWriterChannel chan *FileWriterMessage

type FileWriterMessage struct {
	filename string
	offset   int64
	data     []byte
	torrent  *Torrent
}

func resendFileWriterMessage(message *FileWriterMessage) {
	fileWriterChannel <- message
}

func fileWriterListener() {
	for {
		message := <-fileWriterChannel

		file, err := os.OpenFile(message.filename, os.O_WRONLY, 0600)
		if err != nil {
			if os.IsNotExist(err) {
				if file, err = os.Create(message.filename); err != nil {
					//log.Printf("Failed to create file %s: %s", message.filename, err)
					go resendFileWriterMessage(message)
					continue
				}
			} else {
				//log.Printf("Failed to open file %s: %s", message.filename, err)
				go resendFileWriterMessage(message)
				continue
			}
		}

		if _, err := file.WriteAt(message.data, message.offset); err != nil {
			//log.Printf("Failed to write to file %s: %s", message.filename, err)
			go resendFileWriterMessage(message)
			continue
		}

		file.Close()

		go func(fileWriteDone chan struct{}) {
			fileWriteDone <- struct{}{}
		}(message.torrent.fileWriteDone)
	}
}
