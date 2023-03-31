package logging

import (
	logr "github.com/sirupsen/logrus"
)

type LegalNotice struct {
	logger *logr.Logger
	text   string
}

func NewLegalNotice(logger *logr.Logger) *LegalNotice {
	return &LegalNotice{
		logger: logger,
		text:   "\n\n*************************************\n\nThis software is copyright of Armory, Inc. Any use of this software is subject to and governed by the Armory Terms and Conditions available at https://www.armory.io/terms-and-conditions/ and this software is considered part of Armory Services under those Terms and Conditions.\n\nYour continued use of this software acknowledges and accepts the Armory Terms and Conditions.\n\nUse of this software requires a valid license which can be obtained from Armory by contacting info@armory.io.\n\n*************************************\n\n",
	}
}

func (o *LegalNotice) PrintLegalNotice() {
	o.logger.Warnf(o.text)
}
