package tcp

type renoState struct {
	s *sender
}

// 新建 reno 算法对象
func newRenoCC(s *sender) *renoState {
	return &renoState{s: s}
}

// updateSlowStart 将根据NewReno使用的慢启动算法更新拥塞窗口。
// 如果在调整拥塞窗口后我们越过了 SSthreshold ，那么它将返回在拥塞避免模式下必须消耗的数据包的数量。
func (r *renoState) updateSlowStart(packetsAcked int) int {
	// 在慢启动阶段，每次收到ack，sndCwnd加上已确认的段数
	newcwnd := r.s.sndCwnd + packetsAcked
	// 判断增大过后的拥塞窗口是否超过慢启动阀值 sndSsthresh，
	// 如果超过 sndSsthresh ，将窗口调整为 sndSsthresh
	if newcwnd >= r.s.sndSsthresh {
		newcwnd = r.s.sndSsthresh
		r.s.sndCAAckCount = 0
	}
	// 是否超过 sndSsthresh， packetsAcked>0表示超过
	packetsAcked -= newcwnd - r.s.sndCwnd
	// 更新拥塞窗口
	r.s.sndCwnd = newcwnd
	return packetsAcked
}

// updateCongestionAvoidance 在拥塞避免模式下更新拥塞窗口
func (r *renoState) updateCongestionAvoidance(packetsAcked int) {
	// sndCAAckCount 累计收到的tcp段数
	r.s.sndCAAckCount += packetsAcked
	// 如果累计的段数超过当前的拥塞窗口，那么 sndCwnd 加上 sndCAAckCount/sndCwnd 的整数倍
	if r.s.sndCAAckCount >= r.s.sndCwnd {
		r.s.sndCwnd += r.s.sndCAAckCount / r.s.sndCwnd
		r.s.sndCAAckCount = r.s.sndCAAckCount % r.s.sndCwnd
	}
}

// 当检测到网络拥塞时，调用 reduceSlowStartThreshold。
// 它将 sndSsthresh 变为 outstanding 的一半。
// sndSsthresh 最小为2，因为至少要比丢包后的拥塞窗口（cwnd=1）来的大，才会进入慢启动阶段。
func (r *renoState) reduceSlowStartThreshold() {
	r.s.sndSsthresh = r.s.outstanding / 2
	if r.s.sndSsthresh < 2 {
		r.s.sndSsthresh = 2
	}
}

// Update 根据packetsAcked数更新拥塞状态
// packetsAcked 表示已确认的tcp段数
func (r *renoState) Update(packetsAcked int) {
	// 当拥塞窗口没有超过慢启动阀值的时候，使用慢启动来增大窗口，
	// 否则进入拥塞避免阶段
	if r.s.sndCwnd < r.s.sndSsthresh {
		packetsAcked = r.updateSlowStart(packetsAcked)
		if packetsAcked == 0 {
			return
		}
	}
	// 进入拥塞避免阶段
	r.updateCongestionAvoidance(packetsAcked)
}

// HandleNDupAcks 当收到三个重复ack时，调用 HandleNDupAcks 来处理。
func (r *renoState) HandleNDupAcks() {
	// 减小慢启动阀值
	r.reduceSlowStartThreshold()
}

// HandleRTOExpired 当当发生重传包时，调用 HandleRTOExpired 来处理。
func (r *renoState) HandleRTOExpired() {
	// 减小慢启动阀值
	r.reduceSlowStartThreshold()

	// 更新拥塞窗口为1，这样就会重新进入慢启动
	r.s.sndCwnd = 1
}

// PostRecovery implements congestionControl.PostRecovery.
func (r *renoState) PostRecovery() {
	// noop.
}