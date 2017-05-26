package rails

type Workers []*Worker

type Worker struct {
	id        int
	Home      *Station
	workplace *Station
	workTime  int
	in        *Train
	Done      chan bool
	Ready     chan bool // if sits at home waiting for work to do
}

func NewWorker(id int, home *Station, workTime int) (worker *Worker) {
	worker = &Worker{
		id:        id,
		Home:      home,
		workplace: nil,
		workTime:  workTime,
		in:        nil,
		Done:      make(chan bool),
		Ready:     make(chan bool)}
	worker.Ready <- true
	return
}

func (w *Worker) ID() int { return w.id }

func (w *Worker) GoToWork(place *Station, workTime int) {
	w.workplace = place
	w.workTime = workTime

	depT := w.Home.Trains
	arrT := w.workplace.Trains

	for _, dt := range depT {
		for _, at := range arrT {
			if dt == at {
				w.travel(at, w.Home, w.workplace)
				w.work()
				w.travel(at, w.workplace, w.Home)
				return
			}
		}
	}

	// no direct connection, look for change

	for _, dt := range depT {
		for _, sd := range dt.Connects {
			for _, at := range arrT {
				for _, sa := range at.Connects {
					if sd == sa {
						w.travel(dt, w.Home, sd)
						w.travel(at, sd, w.workplace)
						w.work()
						w.travel(at, w.workplace, sa)
						w.travel(dt, sa, w.Home)
						return
					}
				}
			}
		}
	}
}

type Tickets []*Ticket

type Ticket struct {
	owner       *Worker
	departure   *Station
	destination *Station
	train       *Train
}

func (w *Worker) travel(train *Train, from *Station, to *Station) {
	// TODO: travel...
	*from.TicketsFor[train] = append(*from.TicketsFor[train], &Ticket{
		owner:       w,
		departure:   from,
		destination: to,
		train:       train})
	<-w.Done
}

func (w *Worker) work() {
	// TODO: work...
}
