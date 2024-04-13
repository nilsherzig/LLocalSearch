package e2e_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/nilsherzig/LLocalSearch/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main", Ordered, func() {
	Describe("tests one shot questions with known answers", func() {
		// BeforeAll(func() {
		// 	cmd := exec.Command("make", "dev-bg")
		// 	_, err := Run(cmd)
		// 	if err != nil {
		// 		if exitErr, ok := err.(*exec.ExitError); ok {
		// 			GinkgoWriter.Printf("Stderr: %s\n", string(exitErr.Stderr))
		// 		}
		// 	}
		//
		// 	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("start command '%s' failed", cmd.String()))
		//
		// 	DeferCleanup(func() {
		// 		cmd := exec.Command("make", "dev-bg-stop")
		// 		_, err := Run(cmd)
		// 		Expect(err).NotTo(HaveOccurred())
		// 	})
		// })

		It("should be able to get the modellist endpoint", func() {
			Eventually(func() error {
				req, err := http.NewRequest("GET", "http://localhost:3000/api/models", nil)
				Expect(err).ToNot(HaveOccurred())

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return err
				}
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("status code not 200")
				}
				return nil
			}, "2m", "5s").Should(Not(HaveOccurred()))

		})

		defaultModel := "adrienbrault/nous-hermes2pro:Q8_0"
		// defaultModel = "command-r:35b-v0.1-q4_0"

		sessionString := "default"

		DescribeTable("questions and answers", func(prompt string, answerSubstring string, modelname string) {
			requestUrl := fmt.Sprintf("http://localhost:3000/api/stream?prompt=%s&session=%s&modelname=%s", url.QueryEscape(prompt), url.QueryEscape(sessionString), url.QueryEscape(modelname))
			resp, err := http.Get(requestUrl)
			Expect(err).ToNot(HaveOccurred(), "stream request failed")
			Expect(resp.StatusCode).To(Equal(http.StatusOK), "stream request http status code not 200")

			reader := bufio.NewReader(resp.Body)
		inner:
			for {
				line, err := reader.ReadString('\n')
				if err == io.EOF {
					break
				}
				line = strings.TrimLeft(line, "data: ")
				var streamElem utils.HttpJsonStreamElement
				err = json.Unmarshal([]byte(line), &streamElem)
				if err != nil {
					continue
				}
				// TOOD Needed for follow-up quesions
				// if streamElem.Session != sessionString {
				// 	sessionString = streamElem.Session
				// 	continue
				// }
				if streamElem.StepType == utils.StepHandleAgentFinish {
					GinkgoWriter.Printf("line: %s\n", line)
					Expect(streamElem.Message).To(ContainSubstring(answerSubstring), "answer substring not found in response")
					break inner
				}
			}
		},
			Entry("German quesion", "Wann beginnt das Sommersemester an der Hochschule Stralsund?", "März", defaultModel),
			Entry("Fact 1 question", "how much do OpenAI and Microsoft plan to spend on their new datacenter?", "$100 billion", defaultModel),
			Entry("Fact 2", "how much does Obsidian sync cost?", "$4", defaultModel),
		)
	})
})

func Run(cmd *exec.Cmd) ([]byte, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	fmt.Fprintf(GinkgoWriter, "running: %s\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s failed with error: (%v) %s", command, err, string(output))
	}

	return output, nil
}

func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.Replace(wd, "/backend/e2e", "", -1)
	return wd, nil
}
