package router

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log-forwarder-client/database"
	"log-forwarder-client/output"
	"log-forwarder-client/util"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type RetryData struct {
	Outputs  []output.Output
	LineData util.Event
}

type RetryQueue struct {
	logger *slog.Logger
	doneCh chan struct{}
	queue  []RetryData
	db     *sql.DB
	mu     sync.Mutex
}

func NewRetryQueue(logger *slog.Logger) *RetryQueue {
	return &RetryQueue{
		doneCh: make(chan struct{}),
		queue:  []RetryData{},
		logger: logger,
		db:     database.GetDB(),
	}
}

func (rq *RetryQueue) AddRetryData(data util.Event, outputs []output.Output) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	rd := RetryData{
		LineData: data,
		Outputs:  outputs,
	}
	rq.queue = append(rq.queue, rd)
}

func (rq *RetryQueue) RetryHandlerLoop(routerID int64) {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if len(rq.queue) > 100 {
				rq.logger.Warn("More than 100 elements in RetryQueue", "amount", len(rq.queue))
			}

			var remainingData []RetryData
			for _, data := range rq.queue {
				rq.mu.Lock()
				allSucceeded := true
				var failedOutputs []output.Output

				for _, output := range data.Outputs {
					err := output.Write(data.LineData)
					if err != nil {
						allSucceeded = false
						failedOutputs = append(failedOutputs, output)
					}
				}

				if !allSucceeded {
					data.Outputs = failedOutputs
					remainingData = append(remainingData, data)
					err := rq.UpdateOutputsInDB(data, routerID, failedOutputs)
					if err != nil {
						rq.logger.Error("coundnt update outputs in retry_data", "error", err, "routerID", routerID)
					}
				} else {
					err := rq.SetSuccessStateInDB(data, routerID)
					if err != nil {
						rq.logger.Error("coundnt set success state in retry_data", "error", err, "routerID", routerID)
					}
				}
				rq.mu.Unlock()
			}

			// Update the queue with only the remaining data
			rq.queue = remainingData

		case <-rq.doneCh:
			return
		}
	}
}

func (rq *RetryQueue) Stop() {
	close(rq.doneCh)
}

func (rq *RetryQueue) SaveStateToDB(routerID int64) error {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	query := `INSERT OR IGNORE INTO retry_data (data, outputs, router_id) VALUES(?,?,?)`
	for _, data := range rq.queue {
		outputs := BuildOutputDB(data.Outputs)
		processedData, _ := json.Marshal(data.LineData)
		_, err := rq.db.Exec(query, processedData, outputs, routerID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (rq *RetryQueue) UpdateOutputsInDB(data RetryData, routerID int64, newOutputs []output.Output) error {
	processData, _ := json.Marshal(data)
	outputString := BuildOutputDB(newOutputs)
	_, err := rq.db.Exec(`UPDATE retry_data SET outputs = ? WHERE data = ? AND router_id = ?`, outputString, processData, routerID)
	return err
}

func (rq *RetryQueue) SetSuccessStateInDB(data RetryData, routerID int64) error {
	processData, _ := json.Marshal(data)
	_, err := rq.db.Exec(`UPDATE retry_data SET status = true WHERE data = ? AND router_id = ?`, processData, routerID)
	return err
}

func (rq *RetryQueue) LoadDataFromDB(routerID int64, availableOutputs []output.Output) error {
	searchQuery := `Select id, data, outputs FROM retry_data WHERE router_id = ?`
	deleteQuery := `DELETE from retry_data where id = ?`
	res, err := rq.db.Query(searchQuery, routerID)
	if err != nil {
		rq.logger.Error("coundnt load state data from db", "error", err)
		return err
	}

	var malformedDataId []int
	var dataIdsToDelete []int
	for res.Next() {
		var id int
		var queueData RetryData
		var lineDataFromDB []byte
		var outputsFromDB string
		err := res.Scan(&id, &lineDataFromDB, &outputsFromDB)
		if err != nil {
			return err
		}

		outputNames := strings.Split(outputsFromDB, ";")

		for _, outputName := range outputNames {
			for _, output := range availableOutputs {
				if util.GetNameOfInterface(output) == outputName {
					queueData.Outputs = append(queueData.Outputs, output)
				}
			}
		}

		err = json.Unmarshal(lineDataFromDB, &queueData.LineData)
		if err != nil {
			malformedDataId = append(malformedDataId, id)
			continue
		}

		rq.logger.Debug("loaded retry data from db", "data", queueData)

		rq.queue = append(rq.queue, queueData)
		dataIdsToDelete = append(dataIdsToDelete, id)
	}

	res.Close()

	for _, id := range dataIdsToDelete {
		_, err := rq.db.Exec(deleteQuery, id)
		if err != nil {
			rq.logger.Error("coundnt delete dataId from retry_data table", "error", err, "id", id)
		}
	}

	if len(malformedDataId) > 0 {
		return fmt.Errorf("There is malformed data. %v", malformedDataId)
	}

	return nil
}

func BuildOutputDB(outputs []output.Output) string {
	out := ""
	for _, output := range outputs {
		out += util.GetNameOfInterface(output)
		out += ";"
	}
	return out[:len(out)-1]
}
