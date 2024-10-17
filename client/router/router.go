package router

import (
	"context"
	"database/sql"
	"log-forwarder-client/database"
	"log-forwarder-client/filter"
	"log-forwarder-client/input"
	"log-forwarder-client/output"
	"log-forwarder-client/parser"
	"log-forwarder-client/utils"
	"log/slog"
	"sync"
	"time"
)

type Router struct {
	input      input.Input
	parser     parser.Parser
	filter     filter.Filter
	ctx        context.Context
	wg         *sync.WaitGroup
	cancel     context.CancelFunc
	logger     *slog.Logger
	retryQueue *RetryQueue
	outputs    []output.Output
	db         *sql.DB
	dbID       int64
}

func NewRouter(inWg *sync.WaitGroup, parentCtx context.Context, logger *slog.Logger) *Router {
	ctx, cancel := context.WithCancel(parentCtx)
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

func (r *Router) SetParser(parser parser.Parser) {
	if r.parser != nil {
		r.logger.Warn("More than one Parser is for an input defiend")
		return
	}
	r.parser = parser
}

func (r *Router) SetFilter(filter filter.Filter) {
	if r.filter != nil {
		r.logger.Warn("More than one Filter is for an input defiend")
		return
	}
	r.filter = filter
}

func (r *Router) StartHandlerLoop() {
	defer r.wg.Done()
	// Read from input and route to outputs
	for data := range r.input.Read() {
		// Apply parser
		parsedData, err := r.parser.Apply(data)
		if err != nil {
			r.logger.Warn("Coundnt parse input data", "error", err, "data", data)
			continue
		}

		// Apply filter
		if r.filter != nil {
			var pass bool
			parsedData, pass = r.filter.Apply(parsedData)
			if !pass {
				continue
			}
		}

		// If the data passes all filters, send it to outputs
		statusOutputs := []output.Output{}
		for _, output := range r.outputs {
			err = output.Write(parsedData)
			if err != nil {
				statusOutputs = append(statusOutputs, output)
			}

			// Add data to retryQueue if necassary
			if len(statusOutputs) > 0 {
				r.retryQueue.AddRetryData(parsedData, statusOutputs)
			}
		}
	}
}

func (r *Router) Start() {
	if r.input == nil {
		r.logger.Error("Coundnt start router no input is defiend")
		return
	}
	if r.parser == nil {
		r.logger.Error("Coundnt start router no parser is defiend")
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
			r.logger.Warn("didnt start router", "output", utils.GetNameOfInterface(r.outputs[0]), "parser", utils.GetNameOfInterface(r.parser), "parser", utils.GetNameOfInterface(r.parser))
			return
		}
	}

	err = r.retryQueue.LoadDataFromDB(r.dbID, r.outputs)
	if err != nil {
		r.logger.Error("foo", "error", err)
	}

	r.wg.Add(1)
	r.input.Start()
	go r.StartHandlerLoop()
	go r.retryQueue.RetryHandlerLoop(r.dbID)
	go r.StateHandlerLoop()
}

func (r *Router) Stop() {
	r.cancel()
	r.retryQueue.Stop()
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
	r.input.SaveState()
	err := r.retryQueue.SaveStateToDB(r.dbID)
	if err != nil {
		r.logger.Error("coundnt save retryQueue state", "error", err)
	}
}

func (r *Router) RouterSaveStateToDB() error {
	query := `INSERT INTO router (output, input, parser, filter) VALUES (?,?,?,?)`
	output := BuildOutputDB(r.outputs)
	var filter interface{}
	filter = utils.GetNameOfInterface(r.filter)
	if filter == "" {
		filter = nil
	}
	result, err := r.db.Exec(query, output, utils.GetNameOfInterface(r.input), utils.GetNameOfInterface(r.parser), filter)
	if err == nil {
		r.dbID, _ = result.LastInsertId()
	}
	return err
}

func (r *Router) RouterGetIdFromDB() error {
	query := `SELECT id FROM router where output = ? AND input = ? AND parser = ?`
	output := BuildOutputDB(r.outputs)
	result := r.db.QueryRow(query, output, utils.GetNameOfInterface(r.input), utils.GetNameOfInterface(r.parser))
	var id int64
	err := result.Scan(&id)
	if err == nil {
		r.dbID = id
		return nil
	}
	return err
}
