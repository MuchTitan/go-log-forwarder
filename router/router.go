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
	retryQueue *RetryQueue
	db         *sql.DB
	parsers    []parser.Parser
	filters    []filter.Filter
	outputs    []output.Output
	dbID       int64
}

func NewRouter() *Router {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	return &Router{
		wg:         wg,
		ctx:        ctx,
		cancel:     cancel,
		retryQueue: NewRetryQueue(),
		db:         database.GetDB(),
	}
}

func (r *Router) AddParser(parser parser.Parser) {
	r.parsers = append(r.parsers, parser)
}

func (r *Router) AddFilter(filter filter.Filter) {
	r.filters = append(r.filters, filter)
}

func (r *Router) AddOutput(output output.Output) {
	r.outputs = append(r.outputs, output)
}

func (r *Router) SetInput(input input.Input) {
	if r.input != nil {
		slog.Warn("More than one Input is for an router defiend")
		return
	}
	r.input = input
}

func (r *Router) GetInputTag() string {
	return r.input.GetTag()
}

func (r *Router) ApplyParsers(data *util.Event) {
	var err error
	for _, parser := range r.parsers {
		err = parser.Apply(data)
		if err == nil {
			return
		}
	}
	if err != nil {
		slog.Warn("Coundnt parse data with any defiend Parser", "InputTag", data.InputTag, "error", err)
	}
}

func (r *Router) ApplyFilters(data *util.Event) bool {
	for _, filter := range r.filters {
		if passed := filter.Apply(data); !passed {
			return false
		}
	}
	return true
}

func (r *Router) StartHandlerLoop() {
	defer r.wg.Done()
	// Read from input and route to outputs
	for data := range r.input.Read() {
		r.ApplyParsers(&data)

		if passed := r.ApplyFilters(&data); !passed {
			slog.Debug("skip sending log data based on filter", "InputTag", data.InputTag)
			continue
		}

		statusOutputs := []output.Output{}
		for _, output := range r.outputs {
			err := output.Write(data)
			if err != nil {
				slog.Debug("Error while sending", "error", err, "output", util.GetNameOfInterface(output))
				statusOutputs = append(statusOutputs, output)
			}

		}

		// Add data to retryQueue if necassary
		if len(statusOutputs) > 0 {
			r.retryQueue.AddRetryData(data, statusOutputs)
		}
	}
}

func (r *Router) Start() {
	if len(r.outputs) < 1 {
		slog.Error("Coundnt start router no outputs are defiend")
		return
	}
	err := r.RouterGetIdFromDB()
	if err != nil {
		err := r.RouterSaveStateToDB()
		if err != nil {
			slog.Error("coundnt save router to db", "error", err)
			return
		}
	}

	err = r.retryQueue.LoadDataFromDB(r.dbID, r.outputs)
	if err != nil {
		slog.Error("coundnt load retryQueue state from db", "error", err)
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
		slog.Error("coundnt save retryQueue state", "error", err)
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
