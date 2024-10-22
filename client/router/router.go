package router

import (
	"context"
	"database/sql"
	"log-forwarder-client/database"
	"log-forwarder-client/filter"
	"log-forwarder-client/input"
	"log-forwarder-client/output"
	"log-forwarder-client/parser"
	"log-forwarder-client/util"
	"log/slog"
	"sync"
	"time"
)

type Router struct {
	input      input.Input
	ctx        context.Context
	wg         *sync.WaitGroup
	cancel     context.CancelFunc
	logger     *slog.Logger
	retryQueue *RetryQueue
	db         *sql.DB
	outputs    []output.Output
	dbID       int64
}

func NewRouter(inWg *sync.WaitGroup, logger *slog.Logger) *Router {
	ctx, cancel := context.WithCancel(context.Background())
	return &Router{
		wg:         inWg,
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		retryQueue: NewRetryQueue(logger),
		db:         database.GetDB(),
	}
}

func (r *Router) AddOutput(output output.Output) {
	r.outputs = append(r.outputs, output)
}

func (r *Router) SetInput(input input.Input) {
	if r.input != nil {
		r.logger.Warn("More than one Input is for an router defiend")
		return
	}
	r.input = input
}

func (r *Router) ApplyParser(data *util.Event) error {
	var err error
	for _, parser := range parser.AvailableParser {
		if util.TagMatch(data.InputTag, parser.GetMatch()) {
			err = parser.Apply(data)
			if err == nil {
				break
			}
		}
	}
	return err
}

func (r *Router) ApplyFilter(data *util.Event) (bool, error) {
	var err error
	pass := true
	for _, filter := range filter.AvailableFilters {
		if util.TagMatch(data.InputTag, filter.GetMatch()) {
			pass, err = filter.Apply(data)
			if err != nil || !pass {
				continue
			}
			break
		}
	}
	return pass, err
}

func (r *Router) StartHandlerLoop() {
	defer r.wg.Done()
	// Read from input and route to outputs
	for data := range r.input.Read() {
		// Apply parser
		err := r.ApplyParser(&data)
		if err != nil {
			r.logger.Warn("Coundnt parse data with any defiend Parser", "InputTag", data.InputTag)
			continue
		}

		// If the data passes all filters, send it to outputs
		// r.logger.Debug("Sending this to outputs", "data", data.ParsedData)
		statusOutputs := []output.Output{}
		for _, output := range r.outputs {
			err := output.Write(data)
			if err != nil {
				statusOutputs = append(statusOutputs, output)
			}

			// Add data to retryQueue if necassary
			if len(statusOutputs) > 0 {
				r.retryQueue.AddRetryData(data, statusOutputs)
			}
		}
	}
}

func (r *Router) Start() {
	if r.input == nil {
		r.logger.Error("Coundnt start router no input is defiend")
		return
	}
	if len(r.outputs) < 1 {
		r.logger.Error("Coundnt start router no outputs are defiend")
		return
	}
	err := r.RouterGetIdFromDB()
	if err != nil {
		err := r.RouterSaveStateToDB()
		if err != nil {
			r.logger.Error("coundnt save router to db", "error", err)
			return
		}
	}

	err = r.retryQueue.LoadDataFromDB(r.dbID, r.outputs)
	if err != nil {
		r.logger.Error("coundnt save state from retryQueue", "error", err)
	}

	r.wg.Add(1)
	r.input.Start()
	go r.StartHandlerLoop()
	go r.retryQueue.RetryHandlerLoop(r.dbID)
	go r.StateHandlerLoop()
}

func (r *Router) Stop() {
	r.input.Stop()
	r.retryQueue.Stop()
	r.cancel()
	r.wg.Wait()
	r.SaveRouterState()
}

func (r *Router) StateHandlerLoop() {
	r.wg.Add(1)
	defer r.wg.Done()

	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.SaveRouterState()
		}
	}
}

func (r *Router) SaveRouterState() {
	err := r.retryQueue.SaveStateToDB(r.dbID)
	if err != nil {
		r.logger.Error("coundnt save retryQueue state", "error", err)
	}
}

func (r *Router) RouterSaveStateToDB() error {
	query := `INSERT INTO router (output, input) VALUES (?,?)`
	output := BuildOutputDB(r.outputs)
	var filter interface{}
	result, err := r.db.Exec(query, output, util.GetNameOfInterface(r.input), filter)
	if err == nil {
		r.dbID, _ = result.LastInsertId()
	}
	return err
}

func (r *Router) RouterGetIdFromDB() error {
	query := `SELECT id FROM router where output = ? AND input = ?`
	output := BuildOutputDB(r.outputs)
	result := r.db.QueryRow(query, output, util.GetNameOfInterface(r.input))
	var id int64
	err := result.Scan(&id)
	if err == nil {
		r.dbID = id
		return nil
	}
	return err
}
