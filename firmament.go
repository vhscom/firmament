package firmament

// Firmament is the primary integration surface for agent harness developers.
// It wires together the expertise layer (Graph, Ground) and the monitoring
// layer (Monitor) into a single entry point.
//
// Minimal harness integration:
//
//	f, _ := firmament.New(cfg)
//	go f.Monitor.Run(ctx)        // monitoring in background
//	g, _ := f.Ground(ctx, task)  // consult graph before each agent task
//	// inject g into agent context
//
// See ADR-005 Decision 7.
type Firmament struct {
	// Graph holds the parsed knowledge graph for this deployment.
	// Empty when no GraphPath is configured; Ground returns zero Groundings.
	Graph *Graph

	// Monitor is the behavioral monitoring infrastructure from ADR-001/002/004.
	// Register additional EventSources and configure TrustStore/SessionStore
	// on Monitor before calling Monitor.Run.
	Monitor *Monitor
}

// New creates a Firmament instance wiring together the expertise layer
// (Graph, Ground) and the monitoring layer (Monitor, patterns).
// If cfg.GraphPath is empty or absent, Graph is initialized empty and
// Ground returns zero Groundings without error.
func New(cfg *Config) (*Firmament, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	graph, err := LoadGraph(cfg.GraphPath)
	if err != nil {
		return nil, err
	}

	mon := NewMonitor()
	for _, name := range cfg.EnabledPatterns() {
		if p := PatternByName(name); p != nil {
			mon.AddPattern(p)
		}
	}

	return &Firmament{
		Graph:   graph,
		Monitor: mon,
	}, nil
}
