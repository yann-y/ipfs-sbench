package conf

import "errors"

type TestConf struct {
	Tests Tests `yaml:"tests"`
}

type Tests struct {
	ThreadCount int    `yaml:"threadCount"`
	Ipfs        string `yaml:"ipfs"`
	Name        string `yaml:"name"`
	File        File   `yaml:"file"`
}

type File struct {
	Size        int    `yaml:"size"`
	Number      int    `yaml:"number"`
	ReadWeight  int    `yaml:"read_weight"`
	WriteWeight int    `yaml:"write_weight"`
	Unit        string `yaml:"unit"`
}

func CheckTestCase(testcase *TestConf) error {
	// 检查ipfs地址是否为空
	if testcase.Tests.Ipfs == "" {
		return errors.New("ipfs地址不能为空")
	}
	if testcase.Tests.ThreadCount <= 0 {
		return errors.New("线程数 不能小于等于0")
	}
	if testcase.Tests.File.Number <= 0 {
		return errors.New("文件数 不能小于等于0")
	}
	if testcase.Tests.File.Size <= 0 {
		return errors.New("文件大小 不能小于等于0")
	}
	//不能全是读取，没有写入
	if testcase.Tests.File.ReadWeight == 100 {
		return errors.New("不能全是读取没有写入")
	}
	// 读取写入相加要等于百分之一百

	return nil
}
