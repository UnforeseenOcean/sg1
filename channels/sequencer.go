/*
* Copyleft 2017, Simone Margaritelli <evilsocket at protonmail dot com>
* Redistribution and use in source and binary forms, with or without
* modification, are permitted provided that the following conditions are met:
*
*   * Redistributions of source code must retain the above copyright notice,
*     this list of conditions and the following disclaimer.
*   * Redistributions in binary form must reproduce the above copyright
*     notice, this list of conditions and the following disclaimer in the
*     documentation and/or other materials provided with the distribution.
*   * Neither the name of ARM Inject nor the names of its contributors may be used
*     to endorse or promote products derived from this software without
*     specific prior written permission.
*
* THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
* AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
* IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
* ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE
* LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
* CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
* SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
* INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
* CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
* ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
* POSSIBILITY OF SUCH DAMAGE.
 */
package channels

import (
	"github.com/evilsocket/sg1"
	"sort"
	"sync"
	"sync/atomic"
)

type Sequencer struct {
	seqn  uint32
	in    chan *Packet
	mutex *sync.Mutex
	cond  *sync.Cond
	queue []*Packet
}

func NewSequencer() *Sequencer {
	s := &Sequencer{
		seqn:  0,
		in:    make(chan *Packet),
		mutex: &sync.Mutex{},
		queue: make([]*Packet, 0),
	}

	s.cond = sync.NewCond(s.mutex)

	go s.worker()

	return s
}

func (s *Sequencer) add(p *Packet) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	sg1.Debug("Adding packet with sequence number %d to queue.\n", p.SeqNumber)

	s.queue = append(s.queue, p)

	sg1.Debug("Sorting %d packets in queue.\n", len(s.queue))

	// sort by sequence number
	sort.Slice(s.queue, func(i, j int) bool {
		return s.queue[i].SeqNumber < s.queue[j].SeqNumber
	})

	s.cond.Signal()
}

func (s *Sequencer) worker() {
	sg1.Debug("Packet sequencer started.\n")

	for {
		packet := <-s.in
		s.add(packet)
	}
}

func (s *Sequencer) Packet(data []byte) *Packet {
	size := len(data)
	packet := NewPacket(s.seqn, uint32(size), data)
	sg1.Debug("Sequencer built a packet with seqn=%d\n", s.seqn)
	atomic.AddUint32(&s.seqn, 1)
	return packet
}

func (s *Sequencer) Add(packet *Packet) {
	s.in <- packet
}

func (s *Sequencer) HasPacket() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return len(s.queue) > 0
}

func (s *Sequencer) Wait() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.cond.Wait()
}

func (s *Sequencer) WaitForSeqn(n uint32) {
	sg1.Debug("Waiting for packet with sequence number %d.\n", n)

	for {
		if s.HasPacket() == false {
			s.Wait()
		}

		s.mutex.Lock()
		pkt := s.queue[0]
		s.mutex.Unlock()

		if pkt.SeqNumber == n {
			break
		}
	}
}

func (s *Sequencer) Get() *Packet {
	s.WaitForSeqn(s.seqn)
	atomic.AddUint32(&s.seqn, 1)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	packet := s.queue[0]
	s.queue = s.queue[1:]

	sg1.Debug("Returning packet with sequence number %d.\n", packet.SeqNumber)

	return packet
}
