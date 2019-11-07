package pubsub

type SubWrapContext struct {
	IPubSubscriber
	doneFunc func()
}

func SubContextWrap(done func()) *SubWrapContext {
	s := new(SubWrapContext)
	s.doneFunc = done

	return s
}

func (s *SubWrapContext) Wrap(sub IPubSubscriber) IPubSubscriber {
	s.IPubSubscriber = sub
	return s
}

func (s *SubWrapContext) done() {
	s.doneFunc()
	s.IPubSubscriber.done()
}

func (s *SubWrapContext) Subscribe(path string) *SubWrapContext {
	globalSubscribe(path, s)
	return s
}
