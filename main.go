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
type Job func(stopChan <-chan struct{}, args ...interface{}) (result interface{})

var jobCompleted = make(map[string]bool) // เพิ่มตัวแปรสถานะเพื่อติดตามว่างานได้จบลงหรือยัง

type JobDetail struct {
	Name       string
	Args       []interface{}
	Delay      time.Duration
	Trigger    func() bool // เงื่อนไขเริ่มงาน
	Depends    []string    // ชื่องานที่งานนี้ต้องรอให้เสร็จก่อน
	InDepends  []string    // ชื่องานที่ห้ามรันในขณะที่งานนี้กำลังดำเนินการ
	PreDepends []string    // ชื่องานที่ต้องหยุดก่อนที่งานนี้จะเริ่ม
}

type JobStatus struct {
	Running   bool
	StartTime time.Time
	EndTime   time.Time
}

var mu sync.Mutex // Mutex สำหรับการเข้าถึง jobCompletedห
/*
exp:
	// รอจนกว่างานที่เรียกจะเสร็จ
	waitForJobCompletion("anotherJob")
*/

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
func checkCurrentPageAndDecide(stopChan <-chan struct{}, args ...interface{}) (result interface{}) {
	// Implement your logic here.
	jobCompleted["checkCurrentPageAndDecide"] = true // อัปเดตสถานะเมื่องานจบ
	return nil
}

// fetchTransactionData is a mock function.
func fetchTransactionData(stopChan <-chan struct{}, args ...interface{}) (result interface{}) {
	// Implement your logic here.
	jobCompleted["fetchTransactionData"] = true // อัปเดตสถานะเมื่องานจบ
	return nil
}

func combinedWorker(jobs map[string]Job, jobDetails []JobDetail, jobStatuses map[string]*JobStatus) {
	var wg sync.WaitGroup

	for _, jobDetail := range jobDetails {
		wg.Add(1)
		go func(jd JobDetail) {
			defer wg.Done()

			// ตรวจสอบ PreDepends - รอให้งานเหล่านี้หยุดทำงานก่อน
			for _, preDepend := range jd.PreDepends {
				waitForJobCompletion(preDepend)
			}

			stopChan := make(chan struct{})

			// รอจนกว่าเงื่อนไขการเริ่มและ Depends จะถูกต้อง
			for !jd.Trigger() || !areDependenciesCompleted(jd.Depends) {
				time.Sleep(1 * time.Second)
			}

			// ตรวจสอบ InDepends - หยุดงานถ้างานเหล่านี้กำลังทำงาน
			if areAnyJobsRunning(jd.InDepends) {
				return
			}

			// จัดการเวลาหน่วง
			time.Sleep(jd.Delay)

			// จัดการการเริ่มงาน
			job := jobs[jd.Name]
			jobStatuses[jd.Name].Running = true
			jobStatuses[jd.Name].StartTime = time.Now()

			job(stopChan, jd.Args...)

			jobStatuses[jd.Name].Running = false
			jobStatuses[jd.Name].EndTime = time.Now()
			jobCompleted[jd.Name] = true // อัปเดตสถานะเมื่องานจบ
		}(jobDetail)
	}

	wg.Wait()
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

var stealthMode bool

func main() {

	jobs := make(map[string]Job)
	jobStatuses := make(map[string]*JobStatus)

	browser := startBrowser(stealthMode)

	// Define jobs.
	jobs["checkCurrentPageAndDecide"] = checkCurrentPageAndDecide
	jobs["fetchTransactionData"] = fetchTransactionData

	// Define jobDetails.
	jobDetails := []JobDetail{
		{
			Name:  "checkCurrentPageAndDecide",
			Args:  []interface{}{browser},
			Delay: 2 * time.Second,
		},
		{
			Name:  "fetchTransactionData",
			Args:  []interface{}{browser},
			Delay: 5 * time.Second,
		},
	}

	// Initialize job statuses.
	for jobName := range jobs {
		jobStatuses[jobName] = &JobStatus{}
	}

	combinedWorker(jobs, jobDetails, jobStatuses)

	for jobName, status := range jobStatuses {
		fmt.Printf("Job %s started at %v, ended at %v, running: %v\n", jobName, status.StartTime, status.EndTime, status.Running)
	}

	gracefulTermination()
	// browser.MustClose() // Uncomment this if your browser implementation has a MustClose method.
}

func gracefulTermination() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	fmt.Println("Graceful termination initiated.")
	os.Exit(0)
}

/*
// Example function to demonstrate a job that calls another job and waits for its completion.
func jobThatCallsAnotherJob(stopChan <-chan struct{}, args ...interface{}) (result interface{}) {
	// เรียกใช้งานอีกฟังก์ชันหนึ่ง
	go anotherJob(stopChan, args...)

	// รอจนกว่างานที่เรียกจะเสร็จ
	waitForJobCompletion("anotherJob")

	// ดำเนินการต่อหลังจากงานที่เรียกเสร็จ
	fmt.Println("Continuing after anotherJob completed")
	jobCompleted["jobThatCallsAnotherJob"] = true
	return nil
}

// Another job function.
func anotherJob(stopChan <-chan struct{}, args ...interface{}) (result interface{}) {
	time.Sleep(2 * time.Second) // สมมติว่ามีการทำงานบางอย่างที่นี่
	fmt.Println("anotherJob is completed")
	jobCompleted["anotherJob"] = true
	return nil


func main() {
	stopChan := make(chan struct{})
	go jobThatCallsAnotherJob(stopChan, nil)

	// รอจนกว่า jobThatCallsAnotherJob จะเสร็จ
	waitForJobCompletion("jobThatCallsAnotherJob")
	fmt.Println("Main function completed")
}
} */

/*
func combinedWorker(jobs map[string]Job, jobDetails []JobDetail, jobStatuses map[string]*JobStatus) {
	var wg sync.WaitGroup

	for _, jobDetail := range jobDetails {
		wg.Add(1)
		go func(jd JobDetail) {
			defer wg.Done()
			stopChan := make(chan struct{})

			// รอจนกว่าเงื่อนไขการเริ่มจะถูกต้อง
			for !jd.Trigger() {
				time.Sleep(1 * time.Second)
			}

			// จัดการเวลาหน่วง
			time.Sleep(jd.Delay)

			// จัดการการเริ่มงาน
			job := jobs[jd.Name]
			jobStatuses[jd.Name].Running = true
			jobStatuses[jd.Name].StartTime = time.Now()

			job(stopChan, jd.Args...)

			jobStatuses[jd.Name].Running = false
			jobStatuses[jd.Name].EndTime = time.Now()
		}(jobDetail)
	}

	wg.Wait()
}
*/
