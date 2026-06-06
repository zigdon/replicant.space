package cache

type Star struct {
	Designation string
	EntryPoint  string
	EstPlanets  int
	Explored    bool
	HasLife     bool
	Name        string
	PositionX   float32
	PositionY   float32
	PositionZ   float32
}

func (s *Star) Map() map[string]any {
	return map[string]any{
		"designation": s.Designation,
		"entry_point": s.EntryPoint,
		"est_planets": s.EstPlanets,
		"explored":    s.Explored,
		"has_life":    s.HasLife,
		"name":        s.Name,
		"position_x":  s.PositionX,
		"position_y":  s.PositionY,
		"position_z":  s.PositionZ,
	}
}

func (s *Star) Load(scan func(...any) error) error {
	return scan(
		&s.Designation,
		&s.Name,
		&s.EntryPoint,
		&s.EstPlanets,
		&s.Explored,
		&s.HasLife,
		&s.PositionX,
		&s.PositionY,
		&s.PositionZ,
	)
}

func (s *Star) Equal(old *Star) bool {
	return s.Designation == old.Designation &&
		s.Name == old.Name &&
		s.EntryPoint == old.EntryPoint &&
		s.EstPlanets == old.EstPlanets &&
		s.HasLife == old.HasLife &&
		s.PositionX == old.PositionX &&
		s.PositionY == old.PositionY &&
		s.PositionZ == old.PositionZ
}

type Planet struct {
	Designation string
	LifeStage   string
	Moons       int
	Name        string
	Rings       bool
	Scanned     bool
	Star        string
	Type        int
}

func (p *Planet) Load(scan func(...any) error) error {
	return scan(
		&p.Designation,
		&p.Star,
		&p.Name,
		&p.LifeStage,
		&p.Moons,
		&p.Rings,
		&p.Scanned,
		&p.Type,
	)
}

type Moon struct {
	Designation string
	Name        string
	Planet      string
	Scanned     bool
	Star        string
	Type        string
}

func (m *Moon) Load(scan func(...any) error) error {
	return scan(
		&m.Designation,
		&m.Planet,
		&m.Star,
		&m.Name,
		&m.Scanned,
		&m.Type,
	)
}

type Belt struct {
	Density     string
	Designation string
	Resources   []Resource
	Star        string
}

func (b *Belt) Load(scan func(...any) error) error {
	return scan(
		&b.Designation,
		&b.Star,
		&b.Density,
	)
}

type Resource struct {
	Density  string
	Resource string
}
