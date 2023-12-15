//main.go
/*

remark All function must be end status exp: jobCompleted["checkCurrentPageAndDecide"] = true // อัปเดตสถานะเมื่องานจบ

*/
package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/stealth"
)

// Job is a function type that represents a job to be executed.// Job เป็น type ฟังก์ชันที่แทนงานที่จะถูกดำเนินการ
// type Job func(stopChan <-chan struct{}, args ...interface{}) (result interface{})
type Job func(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error)

// ประกาศตัวแปรที่ใช้ในระบบงานทั้งหมด
var (
	//jobCompleted = make(map[string]bool)
	jobStatuses = make(map[string]*JobStatus)
	jobs        = make(map[string]Job)
	mu          sync.Mutex
	showJobLogs = true
)

var jobCompleted sync.Map

// JobDetail ระบุรายละเอียดของแต่ละงาน เช่น ชื่องาน, อาร์กิวเมนต์, เวลาหน่วง, ทริกเกอร์
type JobDetail struct {
	Name       string
	Args       []interface{}
	Delay      time.Duration
	Trigger    func() bool
	Depends    []string
	InDepends  []string
	PreDepends []string
}

// JobStatus จัดเก็บสถานะของงาน รวมถึงเวลาเริ่มและเวลาจบ
type JobStatus struct {
	Running   bool
	StartTime time.Time
	EndTime   time.Time
}

/*
exp:

	// รอจนกว่างานที่เรียกจะเสร็จ
	waitForJobCompletion("anotherJob")
*/
/* func initializeJobStatuses() {
	for jobName := range jobs {
		jobStatuses[jobName] = &JobStatus{
			Running:   false,
			StartTime: time.Time{},
			EndTime:   time.Time{},
		}
	}
} */
// initializeJobStatuses สร้างและเริ่มต้นสถานะงานจาก JobDetail
func initializeJobStatuses(jobDetails []JobDetail) {
	for _, jd := range jobDetails {
		jobStatuses[jd.Name] = &JobStatus{
			Running:   false,
			StartTime: time.Time{},
			EndTime:   time.Time{},
		}
	}
}

// afterJobCompletion สร้างทริกเกอร์ที่จะเปิดใช้งานเมื่องานที่กำหนดเสร็จสมบูรณ์
func afterJobCompletion(jobName string) func() bool {
	return func() bool {
		//return jobCompleted[jobName]
		return isJobCompleted(jobName)
	}
}

/* exp:
{
	Name:    "fetchTransactionData",
	Args:    []interface{}{browser},
	Delay:   5 * time.Second,
	Trigger: afterJobCompletion("checkCurrentPageAndDecide"), // เริ่มหลังจาก checkCurrentPageAndDecide จบ
},
*/
// Mock Browser type for demonstration purposes.// Mock Browser สำหรับแสดงวิธีการใช้งาน
type Browser struct{}

// startBrowser is a mock function to start a browser.
func startBrowser(useStealth bool) *rod.Browser {
	l := launcher.New()
	url := l.MustLaunch()
	browser := rod.New().ControlURL(url).MustConnect()

	if useStealth {
		stealth.MustPage(browser) // ใช้งาน stealth mode
	}

	return browser
}

// checkCurrentPageAndDecide is a mock function.
func checkCurrentPageAndDecide(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	// Implement your logic here.
	//jobCompleted["checkCurrentPageAndDecide"] = true // อัปเดตสถานะเมื่องานจบ
	setJobCompleted("checkCurrentPageAndDecide", true) // Update status correctly
	return nil, nil
}

// fetchTransactionData is a mock function.
func fetchTransactionData(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	// Implement your logic here.
	//jobCompleted["fetchTransactionData"] = true // อัปเดตสถานะเมื่องานจบ
	setJobCompleted("fetchTransactionData", true) // Update status correctly
	return nil, nil
}

// combinedWorker รันงานตาม JobDetail ที่ระบุ
func combinedWorker(jobs map[string]Job, jobDetails []JobDetail, jobStatuses map[string]*JobStatus) {
	var wg sync.WaitGroup
	for _, jobDetail := range jobDetails {
		wg.Add(1)
		go func(jd JobDetail) {
			defer wg.Done()

			if !executeJobWithStatus(jd, jobs, jobStatuses) {
				fmt.Printf("Failed to execute job %s\n", jd.Name)
			}
		}(jobDetail)
	}
	wg.Wait()
}

// การปรับปรุง executeJobWithStatus ให้ป้องกันการ deadlock
func executeJobWithStatus(jd JobDetail, jobs map[string]Job, jobStatuses map[string]*JobStatus) bool {
	mu.Lock()
	jobStatus, exists := jobStatuses[jd.Name]
	if !exists || jobStatus == nil {
		jobStatus = &JobStatus{}
		jobStatuses[jd.Name] = jobStatus
	}
	mu.Unlock()

	jobFunc, ok := jobs[jd.Name]
	if !ok {
		fmt.Printf("Job function %s not found\n", jd.Name)
		return false
	}

	// สร้าง stopChan ที่นี่
	stopChan := make(chan struct{})

	// เริ่มต้นงานและอัปเดตสถานะโดยไม่ล็อกในกอรูทีนเดียวกัน
	go func() {
		_, err := jobFunc(stopChan, jd.Args...) // ส่ง stopChan ไปยัง jobFunc
		if err != nil {
			fmt.Printf("Error executing job %s: %v\n", jd.Name, err)
		}
		setJobCompleted(jd.Name, true) // ใช้ฟังก์ชันที่มีการล็อกอย่างถูกต้อง
	}()

	return true
}

func waitForDependencies(dependencies []string) {
	for _, dep := range dependencies {
		waitForJobCompletion(dep)
	}
}

func checkInDepends(inDepends []string) bool {
	for _, inDep := range inDepends {
		if isJobRunning(inDep) {
			return true
		}
	}
	return false
}

func executeJob(jd JobDetail, jobs map[string]Job, jobStatuses map[string]*JobStatus) {
	stopChan := make(chan struct{})

	mu.Lock()
	if jobStatus, exists := jobStatuses[jd.Name]; exists && jobStatus != nil {
		jobStatus.Running = true
		jobStatus.StartTime = time.Now()
		if showJobLogs {
			fmt.Printf("Job %s is running...\n", jd.Name)
		}
	} else {
		fmt.Printf("Job %s status not found or nil\n", jd.Name)
	}
	mu.Unlock()

	time.Sleep(jd.Delay)

	jobFunc, ok := jobs[jd.Name]
	if !ok {
		fmt.Printf("Job function %s not found\n", jd.Name)
		return
	}

	_, err := jobFunc(stopChan, jd.Args...)
	if err != nil {
		fmt.Printf("Error executing job %s: %v\n", jd.Name, err)
	}

	mu.Lock()
	if jobStatus, exists := jobStatuses[jd.Name]; exists && jobStatus != nil {
		jobStatus.Running = false
		jobStatus.EndTime = time.Now()
		//jobCompleted[jd.Name] = true
		setJobCompleted(jd.Name, true)
		if showJobLogs {
			fmt.Printf("Job %s has ended.\n", jd.Name)
		}
	} else {
		fmt.Printf("Job %s status not found or nil after execution\n", jd.Name)
	}
	mu.Unlock()
}

func areDependenciesCompleted(dependencies []string) bool {
	mu.Lock()
	defer mu.Unlock()
	for _, dep := range dependencies {
		if done := isJobCompleted(dep); !done {
			return false
		}
	}
	return true
}

func areAnyJobsRunning(jobs []string) bool {
	mu.Lock()
	defer mu.Unlock()
	for _, job := range jobs {
		if running := isJobCompleted(job); running {
			return true
		}
	}
	return false
}

// Function to wait for a job to complete.
func waitForJobCompletion(jobName string) {
	for {
		if isJobCompleted(jobName) {
			break
		}
		time.Sleep(1 * time.Second)
	}
}

// Helper function to check if a job is not started or completed
func isJobNotStartedOrCompleted(jobName string) bool {
	completed, exists := jobCompleted.Load(jobName)
	return !exists || !completed.(bool) // Modified line
}

/* func isJobNotStartedOrCompleted(jobName string) bool {
	mu.Lock()
	defer mu.Unlock()
	_, exists := jobCompleted[jobName]
	return !exists || !jobCompleted[jobName]
}
*/

// Example modification for jobA
func jobA(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobA")
	// Simulate work
	time.Sleep(2 * time.Second)
	setJobCompleted("jobA", true) // Update status with thread-safe function
	return "Job A Result", nil
}

func jobB(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobB")
	// Simulate work
	time.Sleep(3 * time.Second)
	//jobCompleted["jobB"] = true //if some error occor
	setJobCompleted("jobB", true) // Thread-safe update
	return "Job B Result", nil
}

func jobC(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobC")
	// Simulate work or add your logic here.
	time.Sleep(2 * time.Second)
	setJobCompleted("jobC", true) // แทนที่การใช้ jobCompleted["jobC"] = true
	return "Job C Result", nil
}

// ตัวอย่างฟังก์ชัน jobD
func jobD(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	if isJobRunning("jobE") {
		fmt.Println("jobD is waiting because jobE is running")
		return nil, nil
	}
	// ทำงานของ jobD
	fmt.Println("Executing jobD")
	//jobCompleted["jobD"] = true
	setJobCompleted("jobD", true)
	return nil, nil
}

// jobE is a function that requires jobF to be completed before it starts.
func jobE(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobE")
	// Wait for jobF to complete
	waitForJobCompletion("jobF")
	// Implement jobE logic here
	time.Sleep(2 * time.Second)
	//jobCompleted["jobE"] = true
	setJobCompleted("jobE", true)
	return "Job E Result", nil
}

// ตัวอย่างฟังก์ชัน jobF
func jobF(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	waitForJobToStop("jobG")
	// ทำงานของ jobF
	fmt.Println("Executing jobF")
	setJobCompleted("jobF", true)
	return nil, nil
}

// jobG is a function that triggers only if jobD is not running.
func jobG(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobG")
	// Check if jobD is running
	if isJobRunning("jobD") {
		fmt.Println("Job G is waiting for jobD to stop")
		waitForJobToStop("jobD")
	}
	// Implement jobG logic here
	time.Sleep(1 * time.Second)
	setJobCompleted("jobG", true)
	return "Job G Result", nil
}

// ตัวอย่างฟังก์ชัน jobH
func jobH(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	go jobI(stopChan, args...)
	go jobJ(stopChan, args...)
	waitForJobCompletion("jobI")
	waitForJobCompletion("jobJ")
	// ทำงานของ jobH
	fmt.Println("Executing jobH")
	setJobCompleted("jobH", true)
	return nil, nil
}

// jobI is a mock function.
func jobI(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	// Implement your logic here.
	fmt.Println("Executing jobI")
	time.Sleep(2 * time.Second) // สมมติว่ามีการทำงานบางอย่างที่นี่
	setJobCompleted("jobI", true)
	return nil, nil
}

// jobJ is a mock function.
func jobJ(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobJ")
	time.Sleep(3 * time.Second) // สมมติว่ามีการทำงานบางอย่างที่นี่
	setJobCompleted("jobJ", true)
	return nil, nil
}

// jobK is a function that performs a specific task.
// It depends on the completion of jobA and jobB.
func jobK(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobK")

	// Simulate work or add your logic here.
	time.Sleep(2 * time.Second)

	// Update the job completion status.
	setJobCompleted("jobK", true)

	return "Job K Result", nil
}

// สร้างฟังก์ชันงานใหม่สำหรับ Parallel Execution
func jobX(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobX")
	time.Sleep(1 * time.Second) // การทำงาน
	setJobCompleted("jobX", true)
	return "Job X Result", nil
}

func jobY(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobY")
	time.Sleep(2 * time.Second) // การทำงาน
	setJobCompleted("jobY", true)
	return "Job Y Result", nil
}

// ฟังก์ชันงานสำหรับ Non-synchronous Execution
func jobZ(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobZ")
	if isJobCompleted("jobX") {
		fmt.Println("jobZ is waiting for jobX to complete")
		waitForJobCompletion("jobX")
	}
	time.Sleep(1 * time.Second) // การทำงาน
	setJobCompleted("jobZ", true)
	return "Job Z Result", nil
}

func waitForJobToStop(jobName string) {
	for {
		mu.Lock()
		status, exists := jobStatuses[jobName]
		if !exists || !status.Running {
			mu.Unlock()
			break
		}
		mu.Unlock()
		time.Sleep(1 * time.Second)
	}
}

func isJobRunning(jobName string) bool {
	mu.Lock()
	defer mu.Unlock()
	status, exists := jobStatuses[jobName]
	return exists && status.Running
}

//var jobCompletedMu sync.RWMutex

func isJobCompleted(jobName string) bool {
	completed, exists := jobCompleted.Load(jobName)
	return exists && completed.(bool)
}

func setJobCompleted(jobName string, completed bool) {
	jobCompleted.Store(jobName, completed)
}

func updateJobStatus(jobName string, running bool, endTime time.Time) {
	mu.Lock()
	if status, exists := jobStatuses[jobName]; exists {
		status.Running = running
		status.EndTime = endTime
	}
	mu.Unlock()

	setJobCompleted(jobName, !running) // Modified line
}

func main() {

	//ประกาศ var เป็น global ไว้แล้ว แต่ ประกาศซ้ำเพื่อให้รู้ว่า มีอะไรประกาศไว้บ้าง
	//jobCompleted = make(map[string]bool) // เพิ่มตัวแปรสถานะเพื่อติดตามว่างานได้จบลงหรือยัง
	jobStatuses = make(map[string]*JobStatus)
	jobs = make(map[string]Job)
	showJobLogs = true // Set to true to enable job logging, false to disable

	//browser := startBrowser(stealthMode)

	// กำหนดงานและข้อมูลงาน
	jobs["jobA"] = jobA
	jobs["jobB"] = jobB
	jobs["jobC"] = jobC
	jobs["jobD"] = jobD
	jobs["jobE"] = jobE
	jobs["jobF"] = jobF
	jobs["jobG"] = jobG
	jobs["jobH"] = jobH
	jobs["jobI"] = jobI
	jobs["jobJ"] = jobJ
	jobs["jobK"] = jobK

	jobs["jobX"] = jobX
	jobs["jobY"] = jobY
	jobs["jobZ"] = jobZ

	jobs["checkCurrentPageAndDecide"] = checkCurrentPageAndDecide
	jobs["fetchTransactionData"] = fetchTransactionData

	jobs["ControlledJobExecution"] = ControlledJobExecution

	// กำหนดรายละเอียดงานและทริกเกอร์
	jobDetails := []JobDetail{
		{
			Name:       "jobA",
			Args:       []interface{}{},
			Delay:      1 * time.Second,
			Trigger:    func() bool { return true }, // Always trigger
			Depends:    []string{"jobC"},            // Depends on jobC
			InDepends:  []string{},                  // No InDepends
			PreDepends: []string{},                  // No PreDepends
		},
		{
			Name:       "jobB",
			Args:       []interface{}{},
			Delay:      2 * time.Second,
			Trigger:    func() bool { return isJobCompleted("jobA") }, // Trigger after jobA completes
			Depends:    []string{},                                    // No Depends
			InDepends:  []string{"jobD"},                              // Cannot run if jobD is running
			PreDepends: []string{},                                    // No PreDepends
		},
		{
			Name:       "jobC",
			Args:       []interface{}{},
			Delay:      1 * time.Second,
			Trigger:    func() bool { return !isJobRunning("jobB") }, // Trigger if jobB is not running
			Depends:    []string{},                                   // No Depends
			InDepends:  []string{},                                   // No InDepends
			PreDepends: []string{"jobA"},                             // Must stop jobA before starting
		},
		{
			Name:       "jobD",
			Args:       []interface{}{},
			Delay:      1 * time.Second,
			Trigger:    func() bool { return isJobNotStartedOrCompleted("jobB") }, // Trigger if jobB is not started or completed
			Depends:    []string{},                                                // No Depends
			InDepends:  []string{},                                                // No InDepends
			PreDepends: []string{"jobC"},                                          // Must stop jobC before starting
		},

		{
			Name:       "jobE",
			Args:       []interface{}{},
			Delay:      1 * time.Second,
			Trigger:    func() bool { return isJobCompleted("jobF") }, // ทำงานหลังจาก jobF จบ
			Depends:    []string{"jobF"},                              // ขึ้นต่อกับ jobF
			InDepends:  []string{},                                    // ไม่มี InDepends
			PreDepends: []string{},                                    // ไม่มี PreDepends
		},

		{Name: "jobF", Args: []interface{}{}, Delay: 1 * time.Second},

		{
			Name:       "jobG",
			Args:       []interface{}{},
			Delay:      1 * time.Second,
			Trigger:    func() bool { return !isJobRunning("jobD") }, // ทำงานเมื่อ jobD ไม่ทำงาน
			Depends:    []string{},                                   // ไม่มี Depends
			InDepends:  []string{"jobD"},                             // ไม่ทำงานหาก jobD ทำงานอยู่
			PreDepends: []string{},                                   // ไม่มี PreDepends
		},

		{Name: "jobH", Args: []interface{}{}, Delay: 1 * time.Second},

		{
			Name:  "jobI",
			Args:  []interface{}{}, // อาร์กิวเมนต์ของ jobI
			Delay: 1 * time.Second, // หน่วงเวลาตามที่ต้องการ
			// สามารถกำหนด Depends, InDepends หรือ PreDepends ได้ตามต้องการ
		},

		{
			Name:  "jobJ",
			Args:  []interface{}{}, // อาร์กิวเมนต์ของ jobJ
			Delay: 1 * time.Second, // หน่วงเวลาตามที่ต้องการ
			// สามารถกำหนด Depends, InDepends หรือ PreDepends ได้ตามต้องการ
		},

		{
			Name:       "jobK",
			Args:       []interface{}{},
			Delay:      1 * time.Second,
			Trigger:    func() bool { return true }, // Always trigger
			Depends:    []string{"jobA", "jobB"},    // Depends on jobA and jobB
			InDepends:  []string{},                  // No InDepends
			PreDepends: []string{},                  // No PreDepends
		},
	}

	jobDetails = append(jobDetails, JobDetail{Name: "jobX", Args: []interface{}{}, Delay: 0, Trigger: func() bool { return true }})
	jobDetails = append(jobDetails, JobDetail{Name: "jobY", Args: []interface{}{}, Delay: 0, Trigger: func() bool { return true }})
	jobDetails = append(jobDetails, JobDetail{Name: "jobZ", Args: []interface{}{}, Delay: 0, Trigger: func() bool { return !isJobCompleted("jobX") }})
	// Initialize job statuses with jobDetails

	initializeJobStatuses(jobDetails)

	// รัน combinedWorker
	combinedWorker(jobs, jobDetails, jobStatuses)
	// ตรวจสอบสถานะงานหลังจากทำงานเสร็จ
	for jobName, status := range jobStatuses {
		fmt.Printf("Job %s started at %v, ended at %v, running: %v\n", jobName, status.StartTime, status.EndTime, status.Running)
	}

	gracefulTermination()
}

var stealthMode bool

func gracefulTermination() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	fmt.Println("Graceful termination initiated.")
	os.Exit(0)
}

// Global flag to control job execution
var controlFlag bool = false

func pauseOtherJobs() {
	// Set the controlFlag to signal jobs to pause
	controlFlag = true
}

func resumeOtherJobs() {
	// Reset the controlFlag to signal jobs to resume
	controlFlag = false
}

// GOD MODE STOP EVERY JOB (MUST HAVE IF )
// ControlledJobExecution runs its own job in a finite loop and controls other jobs based on controlFlag.
func ControlledJobExecution(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	for i := 0; i < 10; i++ { // Finite loop, replace 10 with the desired number of iterations
		select {
		case <-stopChan:
			return "Stopped", nil
		default:
			fmt.Println("Executing ControlledJobExecution iteration", i)

			// Control other jobs based on the flag
			if controlFlag {
				pauseOtherJobs()
			} else {
				resumeOtherJobs()
			}

			// Simulate work
			time.Sleep(1 * time.Second)
		}
	}
	return "Completed", nil
}

// exp:

// In each job function
func jobAAA(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	for {
		// Check if the job should pause
		if controlFlag {
			// Wait until the flag is reset to false
			for controlFlag {
				time.Sleep(1 * time.Second) // Wait a bit before checking again
			}
		}
		// Job logic here...
	}
}

/*
func main() {
    // ... Your existing code ...

    // Add the ControlledJobExecution to your jobs map
    jobs["ControlledJobExecution"] = ControlledJobExecution

    // ... Rest of your main function ...
}
*/

//////////////////////////////////DDDDOOOOCCCC//////////////////////////////////////////////////////////
/*
To create scenarios that might lead to Deadlock or Errors in the structure of jobDetails in your Go program, you can set conflicting conditions in Depends, InDepends, or PreDepends among different jobs. Here are several ways you can achieve this:

1 Circular Dependencies:
Create a situation where one job depends on another, and vice versa, making it impossible to start or finish either. For example, jobA depends on jobB, and jobB depends on jobA.

2 Conflicts in InDepends:
Set a job to have InDepends that waits for another job to complete, but that job never starts or is unable to complete. For example, jobA has InDepends with jobB, but jobB never starts.

3 Extremely Long Delays:
Set an exceptionally long Delay for a job, causing other jobs that depend on it to wait an excessively long time, or seeming like the program is stuck.

4 Unrealistic Triggers:
Set a Trigger in jobDetails with conditions that are never true or occur. For example, setting a condition that waits for a job that doesn’t exist in jobs.

5 Dependence on Non-Starting Jobs (PreDepends):
Set PreDepends for a job that waits for another job to finish before starting, but the job it depends on might never start or never finish.

Setting these conditions can lead to a deadlock in the program operation, making the program unable to proceed as usual or even get stuck.

Here's an example in Go to demonstrate a circular dependency which can lead to a deadlock:

go
Copy code
type JobDetail struct {
    // ... other fields ...
    Depends []string
}

func main() {
    jobDetails := []JobDetail{
        {
            Name: "jobA",
            // other fields ...
            Depends: []string{"jobB"}, // jobA depends on jobB
        },
        {
            Name: "jobB",
            // other fields ...
            Depends: []string{"jobA"}, // jobB depends on jobA
        },
    }

    // The rest of your main function...
}
In this example, jobA depends on jobB to complete, and jobB depends on jobA. Since neither can start without the other finishing, this creates a deadlock. */

/////////////////////////////////////////////////////////////////////////////////////////////

/*

สถานการณ์ที่อาจนำไปสู่การเกิด Deadlock หรือ Error ในโครงสร้างของ jobDetails ในโปรแกรม Go ของคุณ คุณสามารถกำหนดเงื่อนไขที่ขัดแย้งกันใน Depends, InDepends, หรือ PreDepends ระหว่างงานต่างๆ นี่คือหลายวิธีที่คุณสามารถทำได้:

1 การพึ่งพาที่วงกลม (Circular Dependencies):
สร้างสถานการณ์ที่งานหนึ่งพึ่งพาอีกงานหนึ่งและในทางกลับกัน ทำให้ไม่สามารถเริ่มหรือสิ้นสุดได้ เช่น jobA พึ่งพา jobB และ jobB พึ่งพา jobA.

2 ขัดแย้งใน InDepends:
กำหนดให้งานหนึ่งมี InDepends ที่ต้องรองานอื่นเสร็จสิ้น แต่งานที่มันรอนั้นไม่เคยเริ่มหรือไม่สามารถเสร็จสิ้นได้ เช่น jobA มี InDepends กับ jobB แต่ jobB ไม่เคยเริ่มทำงาน.

3 กำหนด Delay ที่ยาวนานมาก:
กำหนด Delay ที่ยาวนานมากๆ ให้กับงานหนึ่ง ทำให้งานอื่นๆ ที่พึ่งพางานนั้นต้องรอเวลานานมาก หรืออาจจะดูเหมือนว่าโปรแกรมติดขัด.

4 กำหนด Trigger ที่ไม่มีทางเป็นจริง:
กำหนด Trigger ใน jobDetails ที่มีเงื่อนไขที่ไม่มีทางเป็นจริงหรือไม่เคยเกิดขึ้น เช่น ตั้งเงื่อนไขที่ต้องรองานที่ไม่มีอยู่ใน jobs.

5 พึ่งพากับงานที่ไม่มีการทำงาน (PreDepends):
กำหนด PreDepends ในงานหนึ่งที่ต้องรอให้งานอื่นสิ้นสุดก่อนที่จะเริ่มต้น แต่งานที่มันรออาจไม่เคยเริ่มหรือไม่เคยสิ้นสุดได้.

การกำหนดเงื่อนไขเหล่านี้อาจนำไปสู่การติดขัดในการทำงานของโปรแกรม ทำให้โปรแกรมไม่สามารถดำเนินการได้ตามปกติหรือเกิด Deadlock.

*/

/* 		{Name: "jobX", Args: []interface{}{}, Delay: 0, Trigger: func() bool { return true }},
   		{Name: "jobY", Args: []interface{}{}, Delay: 0, Trigger: func() bool { return true }},
   		{Name: "jobZ", Args: []interface{}{}, Delay: 0, Trigger: func() bool { return !isJobCompleted("jobX") }}, */
