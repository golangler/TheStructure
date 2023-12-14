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

// Job is a function type that represents a job to be executed.
// type Job func(stopChan <-chan struct{}, args ...interface{}) (result interface{})
type Job func(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error)

var jobCompleted = make(map[string]bool)
var jobStatuses = make(map[string]*JobStatus)
var jobs = make(map[string]Job)
var showJobLogs = true

type JobDetail struct {
	Name       string
	Args       []interface{}
	Delay      time.Duration
	Trigger    func() bool
	Depends    []string
	InDepends  []string
	PreDepends []string
}

type JobStatus struct {
	Running   bool
	StartTime time.Time
	EndTime   time.Time
}

var mu sync.Mutex

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

func initializeJobStatuses(jobDetails []JobDetail) {
	for _, jd := range jobDetails {
		jobStatuses[jd.Name] = &JobStatus{
			Running:   false,
			StartTime: time.Time{},
			EndTime:   time.Time{},
		}
	}
}

func afterJobCompletion(jobName string) func() bool {
	return func() bool {
		return jobCompleted[jobName]
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
// Mock Browser type for demonstration purposes.
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
	jobCompleted["checkCurrentPageAndDecide"] = true // อัปเดตสถานะเมื่องานจบ
	return nil, nil
}

// fetchTransactionData is a mock function.
func fetchTransactionData(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	// Implement your logic here.
	jobCompleted["fetchTransactionData"] = true // อัปเดตสถานะเมื่องานจบ
	return nil, nil
}

func combinedWorker(jobs map[string]Job, jobDetails []JobDetail, jobStatuses map[string]*JobStatus) {
	var wg sync.WaitGroup

	for _, jobDetail := range jobDetails {
		wg.Add(1)
		go func(jd JobDetail) {
			defer wg.Done()

			// Ensure the jobStatus for this job is initialized
			mu.Lock()
			jobStatus, exists := jobStatuses[jd.Name]
			if !exists || jobStatus == nil {
				jobStatuses[jd.Name] = &JobStatus{}
				jobStatus = jobStatuses[jd.Name]
			}
			mu.Unlock()

			// Check if the job exists
			jobFunc, ok := jobs[jd.Name]
			if !ok {
				fmt.Printf("Job function %s not found\n", jd.Name)
				return
			}

			// Check PreDepends, Wait for Depends, Check InDepends, Wait for Trigger

			stopChan := make(chan struct{})
			_, err := jobFunc(stopChan, jd.Args...)
			if err != nil {
				fmt.Printf("Error executing job %s: %v\n", jd.Name, err)
			}

			// Update job status after completion
			mu.Lock()
			jobStatus.Running = false
			jobStatus.EndTime = time.Now()
			jobCompleted[jd.Name] = true
			mu.Unlock()
		}(jobDetail)
	}
	wg.Wait()
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
		jobCompleted[jd.Name] = true
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
		if done, exists := jobCompleted[dep]; !done || !exists {
			return false
		}
	}
	return true
}

func areAnyJobsRunning(jobs []string) bool {
	mu.Lock()
	defer mu.Unlock()
	for _, job := range jobs {
		if running, exists := jobCompleted[job]; running && exists {
			return true
		}
	}
	return false
}

// Function to wait for a job to complete.
func waitForJobCompletion(jobName string) {
	for {
		mu.Lock()
		if done, exists := jobCompleted[jobName]; done && exists {
			mu.Unlock()
			break
		}
		mu.Unlock()
		time.Sleep(1 * time.Second)
	}
}

// Helper function to check if a job is completed
func isJobCompleted(jobName string) bool {
	mu.Lock()
	defer mu.Unlock()
	return jobCompleted[jobName]
}

// Helper function to check if a job is not started or completed
func isJobNotStartedOrCompleted(jobName string) bool {
	mu.Lock()
	defer mu.Unlock()
	_, exists := jobCompleted[jobName]
	return !exists || !jobCompleted[jobName]
}

var stealthMode bool

func gracefulTermination() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	fmt.Println("Graceful termination initiated.")
	os.Exit(0)
}

// Example modification for jobA
func jobA(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobA")
	// Simulate work
	time.Sleep(2 * time.Second)
	jobCompleted["jobA"] = true
	return "Job A Result", nil
}

func jobB(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobB")
	// Simulate work
	time.Sleep(3 * time.Second)
	jobCompleted["jobB"] = true
	return "Job B Result", nil
}

// It waits for jobA to stop (PreDepends), and it should not run if jobB is running (Trigger condition).
func jobC(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobC")

	// Simulate work or add your logic here.
	time.Sleep(2 * time.Second)

	// Update the job completion status.
	jobCompleted["jobC"] = true

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
	jobCompleted["jobD"] = true
	return nil, nil
}

// ตัวอย่างฟังก์ชัน jobF
func jobF(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	waitForJobToStop("jobG")
	// ทำงานของ jobF
	fmt.Println("Executing jobF")
	jobCompleted["jobF"] = true
	return nil, nil
}

// ตัวอย่างฟังก์ชัน jobH
func jobH(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	go jobI(stopChan, args...)
	go jobJ(stopChan, args...)
	waitForJobCompletion("jobI")
	waitForJobCompletion("jobJ")
	// ทำงานของ jobH
	fmt.Println("Executing jobH")
	jobCompleted["jobH"] = true
	return nil, nil
}

// jobI is a mock function.
func jobI(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	// Implement your logic here.
	fmt.Println("Executing jobI")
	time.Sleep(2 * time.Second) // สมมติว่ามีการทำงานบางอย่างที่นี่
	jobCompleted["jobI"] = true // อัปเดตสถานะเมื่องานจบ
	return nil, nil
}

// jobJ is a mock function.
func jobJ(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobJ")
	time.Sleep(3 * time.Second) // สมมติว่ามีการทำงานบางอย่างที่นี่
	jobCompleted["jobJ"] = true // อัปเดตสถานะเมื่องานจบ
	return nil, nil
}

// jobK is a function that performs a specific task.
// It depends on the completion of jobA and jobB.
func jobK(stopChan <-chan struct{}, args ...interface{}) (result interface{}, err error) {
	fmt.Println("Executing jobK")

	// Simulate work or add your logic here.
	time.Sleep(2 * time.Second)

	// Update the job completion status.
	jobCompleted["jobK"] = true

	return "Job K Result", nil
}

// ... rest of your code ...

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

func main() {
	showJobLogs = true // Set to true to enable job logging, false to disable
	//ประกาศ var เป็น global ไว้แล้ว แต่ ประกาศซ้ำเพื่อให้รู้ว่า มีอะไรประกาศไว้บ้าง
	jobCompleted = make(map[string]bool) // เพิ่มตัวแปรสถานะเพื่อติดตามว่างานได้จบลงหรือยัง
	jobStatuses = make(map[string]*JobStatus)
	jobs = make(map[string]Job)

	//browser := startBrowser(stealthMode)

	// กำหนดงานและข้อมูลงาน
	jobs["jobA"] = jobA
	jobs["jobB"] = jobB
	jobs["jobC"] = jobC
	jobs["jobD"] = jobD
	jobs["jobF"] = jobF
	jobs["jobH"] = jobH
	jobs["jobI"] = jobI
	jobs["jobJ"] = jobJ
	jobs["jobK"] = jobK

	jobs["checkCurrentPageAndDecide"] = checkCurrentPageAndDecide
	jobs["fetchTransactionData"] = fetchTransactionData

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

		{Name: "jobF", Args: []interface{}{}, Delay: 1 * time.Second},
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

	// Initialize job statuses with jobDetails
	initializeJobStatuses(jobDetails)

	// รัน combinedWorker
	combinedWorker(jobs, jobDetails, jobStatuses)
	// ตรวจสอบสถานะงานหลังจากทำงานเสร็จ
	for jobName, status := range jobStatuses {
		fmt.Printf("Job %s started at %v, ended at %v, running: %v\n", jobName, status.StartTime, status.EndTime, status.Running)
	}
}
