package pool

import (
	"errors"
	"fmt"
	"kscan/lib/misc"
	"kscan/lib/slog"
	"kscan/lib/smap"
	"sync"
	"time"
)

//创建worker，每一个worker抽象成一个可以执行任务的函数
type Worker struct {
	f func(interface{}) (interface{}, error)
}

//通过NewTask来创建一个worker
func NewWorker(f func(interface{}) interface{}) *Worker {
	return &Worker{
		f: func(in interface{}) (out interface{}, err error) {
			defer func() {
				if e := recover(); e != nil {
					err = errors.New(fmt.Sprint("param: ", in, e))
					slog.Debug(err, e)
				}
			}()
			out = f(in)
			return out, err
		},
	}
}

//执行worker
func (t *Worker) Run(in interface{}) (interface{}, error) {
	return t.f(in)
}

//池
type Pool struct {
	//母版函数
	Function func(interface{}) interface{}
	//Pool输入队列
	In chan interface{}
	//Pool输出队列s
	Out chan interface{}
	//size用来表明池的大小，不能超发。
	threads int
	//启动协程等待时间
	Interval time.Duration
	//正在执行的任务清单
	JobsList *smap.SMap
	//jobs表示执行任务的通道用于作为队列，我们将任务从切片当中取出来，然后存放到通道当中，再从通道当中取出任务并执行。
	Jobs chan *Worker
	//用于阻塞
	wg *sync.WaitGroup
	//提前结束标识符
	Done bool
}

//实例化工作池使用
func NewPool(threads int) *Pool {
	return &Pool{
		threads:  threads,
		JobsList: smap.New(),
		wg:       &sync.WaitGroup{},
		Out:      make(chan interface{}),
		In:       make(chan interface{}),
		Function: nil,
		Done:     false,
		Interval: time.Duration(0),
	}
}

//从jobs当中取出任务并执行。
func (p *Pool) work() {
	//减少waitGroup计数器的值
	defer func() {
		p.wg.Done()
	}()
	for param := range p.In {
		if p.Done {
			return
		}
		//获取任务唯一票据
		Tick := p.NewTick()
		//压入工作任务到工作清单
		p.JobsList.Set(Tick, param)
		//开始工作
		f := NewWorker(p.Function)
		//开始工作，输出工作结果
		out, err := f.Run(param)
		//工作结束，删除工作清单
		p.JobsList.Delete(Tick)
		if err == nil && out != nil {
			p.Out <- out
		}
		//if err != nil {
		//	panic(err)
		//}
	}
}

//执行工作池当中的任务
func (p *Pool) Run() {
	//只启动有限大小的协程，协程的数量不可以超过工作池设定的数量，防止计算资源崩溃
	for i := 0; i < p.threads; i++ {
		p.wg.Add(1)
		time.Sleep(p.Interval)
		go p.work()
	}
	p.wg.Wait()
	//关闭输出信道
	p.OutDone()
}

//结束输入信道
func (p *Pool) InDone() {
	close(p.In)
}

//结束输出信道
func (p *Pool) OutDone() {
	close(p.Out)
}

//向各工作协程发送提前结束指令
func (p *Pool) Stop() {
	p.Done = true
}

//生成工作票据
func (p *Pool) NewTick() string {
	return misc.RandomString()
}
