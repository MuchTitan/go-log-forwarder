package directory

//
// import (
// 	"encoding/base64"
// 	"encoding/json"
// 	"log-forwarder-client/input"
// 	"log-forwarder-client/utils"
// 	"slices"
// 	"time"
//
// 	"go.etcd.io/bbolt"
// )
//
// func (d *DirectoryState) SaveState(db *bbolt.DB) error {
// 	return db.Update(func(tx *bbolt.Tx) error {
// 		b, err := tx.CreateBucketIfNotExists([]byte(d.path))
// 		if err != nil {
// 			return err
// 		}
//
// 		state := map[string]interface{}{
// 			"Path":              d.path,
// 			"Time":              d.time.Format(time.RFC3339),
// 			"LinesFailedToSend": d.linesFailedToSend,
// 		}
//
// 		// Save running tail states
// 		tails := make(map[string]input.TailFileState)
// 		for path, tail := range d.runningTails {
// 			tailState, err := tail.GetState()
// 			if err != nil {
// 				d.logger.Error("Coundnt get state of file tail", "path", path)
// 				continue
// 			}
// 			tails[path] = tailState
// 		}
// 		state["RunningTails"] = tails
//
// 		encoded, err := json.Marshal(state)
// 		if err != nil {
// 			return err
// 		}
//
// 		d.logger.Debug("saving state", "state", state)
//
// 		return b.Put([]byte("state"), encoded)
// 	})
// }
//
// func (d *DirectoryState) LoadState(db *bbolt.DB) error {
// 	return db.View(func(tx *bbolt.Tx) error {
// 		b := tx.Bucket([]byte(d.path))
// 		if b == nil {
// 			return nil // No state saved yet
// 		}
//
// 		encoded := b.Get([]byte("state"))
// 		if encoded == nil {
// 			return nil // No state saved yet
// 		}
//
// 		var state map[string]interface{}
// 		if err := json.Unmarshal(encoded, &state); err != nil {
// 			return err
// 		}
//
// 		d.path = state["Path"].(string)
//
// 		if parsedTime, err := parseTime(state["Time"].(string)); err != nil {
// 			d.logger.Error("Failed to parse Time", "error", err)
// 			panic(err)
// 		} else {
// 			d.time = parsedTime
// 		}
//
// 		// Safely check if "LinesFailedToSend" exists and is a non-empty slice
// 		if lines, ok := state["LinesFailedToSend"].([]interface{}); ok {
// 			// Convert to [][]byte
// 			var linesFailedToSend [][]byte
// 			for _, line := range lines {
// 				if lineBytes, ok := line.([]byte); ok {
// 					linesFailedToSend = append(linesFailedToSend, lineBytes)
// 				}
// 			}
// 			// assign the converted value to d.LinesFailedToSend
// 			d.linesFailedToSend = linesFailedToSend
// 		}
//
// 		tailStates := d.parseTailsFromDB(state)
//
// 		for filePath, state := range tailStates {
// 			currentInodeNumber, err := utils.GetInodeNumber(filePath)
// 			if err != nil {
// 				continue
// 			}
//
// 			if currentInodeNumber != state.InodeNumber {
// 				d.logger.Debug("InodeNumber changed. Not loading saved state", "path", filePath)
// 				continue
// 			}
//
// 			currentCheckSum, err := utils.CreateChecksumForFirstThreeLines(filePath)
// 			if err != nil {
// 				continue
// 			}
//
// 			if !slices.Equal(currentCheckSum, state.Checksum) {
// 				d.logger.Debug("Checksum for the first 3 lines changed. Not loading saved state", "path", filePath)
// 				continue
// 			}
//
// 			fileTail, err := input.NewTailFile(filePath, state.LastSendLine, d.ctx)
// 			if err != nil {
// 				d.logger.Error("Coundnt start file tail with saved state", "path", filePath, "state", state)
// 				continue
// 			}
//
// 			d.runningTails[filePath] = fileTail
// 		}
//
// 		d.logger.Debug("loading state", "state", state)
// 		return nil
// 	})
// }
//
// func (d *DirectoryState) parseTailsFromDB(state map[string]interface{}) map[string]input.TailFileState {
// 	// Load running tail states
// 	decodedFileTails := make(map[string]input.TailFileState)
// 	runningTails, ok := state["RunningTails"].(map[string]interface{})
// 	if !ok {
// 		return decodedFileTails
// 	}
//
// 	for filePath, fileData := range runningTails {
// 		fileDataMap, ok := fileData.(map[string]interface{})
// 		if !ok {
// 			continue
// 		}
//
// 		var tailState input.TailFileState
//
// 		lastSendLine, ok := fileDataMap["LastSendLine"].(float64)
// 		if !ok {
// 			continue
// 		}
// 		tailState.LastSendLine = int64(lastSendLine) + 1
//
// 		checksumStr, ok := fileDataMap["Checksum"].(string)
// 		if !ok {
// 			continue
// 		}
//
// 		checksum, err := base64.StdEncoding.DecodeString(checksumStr)
// 		if err != nil {
// 			continue
// 		}
// 		tailState.Checksum = checksum
//
// 		inodeNumber, ok := fileDataMap["InodeNumber"].(float64)
// 		if !ok {
// 			continue
// 		}
// 		tailState.InodeNumber = uint64(inodeNumber)
//
// 		decodedFileTails[filePath] = tailState
// 	}
// 	return decodedFileTails
// }
