package service

import "math/rand"

type Quiz struct {
	Question string
	Answer   string
}

var defaultQuizzes = []Quiz{
	{Question: "公司核心价值观第一条是？（请输入：客户至上 / 团队协作 / 创新求变 / 诚信务实 / 极致专注 / 学习成长）", Answer: "客户至上"},
	{Question: "今年企业文化年的主题词是？", Answer: "向上"},
	{Question: "本次活动开始时间（24h 制）整点是？", Answer: "18"},
}

func PickQuiz() Quiz {
	return defaultQuizzes[rand.Intn(len(defaultQuizzes))]
}

func CheckQuiz(expected, answer string) bool {
	return expected == answer
}
