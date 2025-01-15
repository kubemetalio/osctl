package raid

import (
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubemetalio/osctl/pkg/osctl/types"
)

type RaidOptions struct {
	Template types.Template
}

func NewRaidOptions() *RaidOptions {
	return &RaidOptions{}
}

func (o *RaidOptions) AddFlags(fs *pflag.FlagSet) {

}

func (o *RaidOptions) Validate() error {
	allErrs := field.ErrorList{}
	return allErrs.ToAggregate()
}

func (o *RaidOptions) Complete() (err error) {
	return nil
}
