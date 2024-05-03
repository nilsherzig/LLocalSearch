package lschains

import (
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/schema"
)

func RunSourceChainExample() (string, error) {
	sources := []schema.Document{
		{
			PageContent: `Culture Store Forums Settings Front page layout Grid List Site theme light dark Sign in Skynet deferred — OpenAI opens the door for military uses but maintains AI weapons ban Despite new Pentagon collab, OpenAI wont allow customers to develop or use weapons with its tools. Benj Edwards - Jan 17, 2024 9:25 pm UTC EnlargeOpenAI / Getty Images / Benj Edwards reader comments 62 On Tuesday, ChatGPT developer OpenAI revealed that it is collaborating with the United States Defense Department on cybersecurity projects and exploring ways to prevent veteran suicide, reports Bloomberg. OpenAI revealed the collaboration during an interview with the news outlet at the World Economic Forum in Davos. The AI company recently modified its policies, allowing for certain military applications of its technology, while maintaining prohibitions against using it to develop weapons. According to Anna Makanju, OpenAIs vice president of global affairs, many people thought that [a previous blanket prohibition on military applications] would prohibit many of these use cases, which people think are very much aligned with what we want to see in the world.`,
			Metadata: map[string]any{
				"URL": `https://arstechnica.com/information-technology/2024/01/openai-reveals-partnership-with-pentagon-on-cybersecurity-suicide-prevention/`,
			},
		},
		{
			PageContent: `Though, as OpenAI representative Niko Felix explained, there is still a blanket prohibition on developing and using weapons — you can see that it was originally and separately listed from “military and warfare.” After all, the military does more than make weapons, and weapons are made by others than the military. And it is precisely where those categories do not overlap that I would speculate OpenAI is examining new business opportunities. Not everything the Defense Department does is strictly warfare-related; as any academic, engineer or politician knows, the military establishment is deeply involved in all kinds of basic research, investment, small business funds and infrastructure support. OpenAI’s GPT platforms could be of great use to, say, army engineers looking to summarize decades of documentation of a region’s water infrastructure. It’s a genuine conundrum at many companies how to define and navigate their relationship with government and military money. Google’s “Project Maven” famously took one step too far, though few seemed to be as bothered by the multibillion-dollar JEDI cloud contract. It might be OK for an academic researcher on an Air Force Research lab grant to use GPT-4, but not a researcher inside the AFRL working on the same project. Where do you draw the line? Even a strict “no military” policy has to stop after a few removes. That said, the total removal of “military and warfare” from OpenAI’s prohibited uses suggests that the company is, at the very least, open`,
			Metadata: map[string]any{
				"URL": `https://techcrunch.com/2024/01/12/openai-changes-policy-to-allow-military-applications/`,
			},
		},
		{
			PageContent: `Collab OpenAI Lifts Military Ban, Opens Doors to DOD for Cybersecurity Collab Charles Lyons-BurtJanuary 22, 2024Artificial Intelligence,Cybersecurity,News-comment At the World Economic Forum in Davos, Switzerland on Jan. 16, it was revealed that OpenAI and the Department of Defense will be collaborating on artificial intelligence-based cybersecurity technology. The news has broader implications than just those in the cyber or AI realms: before last week, OpenAI had resisted sanctioning use of its popular ChatGPT application by the defense industry and military. However, just days prior to the WEF, this policy was altered, reported The Register. The Potomac Officers Club will host its 2024 Cyber Summit on June 6. Register here (at the Early Bird rate!) to save a spot at what’s sure to be an essential gathering on all things cyber and government contracting, discussing the biggest issues of the day—such as the partnership between the Pentagon and OpenAI. Using ChatGPT for “military and warfare” operations was explicitly barred prior to last week, but their restriction no longer appears in the app’s permissions statement. An OpenAI representative said that the expansion is due to the recognition of “national security use cases that align with our mission.” According to OpenAI Vice President of Global Affairs Anna Makanju, the DOD collaboration intends to produce open-source cybersecurity software. But the company cautioned that its product`,
			Metadata: map[string]any{
				"URL": `https://www.govconwire.com/2024/01/openai-lifts-military-ban-opens-doors-to-dod-for-cybersecurity-collab/`,
			},
		},
		{
			PageContent: `Contribute Become a Wordsmith Share your voice, and win big in blogathons Become a Mentor Craft careers by sharing your knowledge Become a Speaker Inspire minds, share your expertise Become an Instructor Shape next-gen innovators through our programs Corporate Our Offerings Build a data-powered and data-driven workforce Trainings Bridge your teams data skills with targeted training Analytics maturity Unleash the power of analytics for smarter outcomes Data Culture Break down barriers and democratize data access and usage Login Logout d : h : m : s Home Artificial Intelligence OpenAI Works with U.S. Military Soon After Policy Update OpenAI Works with U.S. Military Soon After Policy Update K K. C. Sabreena Basheer 18 Jan, 2024 • 2 min read Aligning with a recent policy shift, OpenAI, the creator of ChatGPT, is actively engaging with the U.S. military on various projects, notably focusing on cybersecurity capabilities. This development comes on the heels of OpenAI’s recent removal of language in its terms of service that previously restricted the use of its artificial intelligence (AI) in military applications. While the company maintains a ban on weapon development and harm, this collaboration underscores a broader update in policies to adapt to new applications of their technology. Also Read: Open`,
			Metadata: map[string]any{
				"URL": `https://www.analyticsvidhya.com/blog/2024/01/openai-works-with-u-s-military-soon-after-policy-update`,
			},
		},
	}
	llm, err := ollama.New(
		ollama.WithModel("llama3:8b-instruct-q6_K"),
		ollama.WithServerURL("http://gpu-ubuntu:11434"),
		ollama.WithRunnerNumCtx(8*1024),
	)
	if err != nil {
		return "", err
	}
	text := `OpenAI is collaborating with the United States Defense Department on cybersecurity projects and exploring ways to prevent veteran suicide. This collaboration was revealed during an interview at the World Economic Forum in Davos. OpenAI recently modified its policies, allowing for certain military applications of its technology while maintaining prohibitions against using it to develop weapons. According to Anna Makanju, OpenAIs vice president of global affairs, many people thought that a previous blanket prohibition on military applications would prohibit many use cases that are aligned with what people want to see in the world.

    Opens Doors to DOD for Cybersecurity Collab`
	return RunSourceChain(llm, sources, text)
}
