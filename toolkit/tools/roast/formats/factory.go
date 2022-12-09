package formats

import (
	"fmt"
)

func ConverterFactory(formatType string) (converter Converter, err error) {
	switch formatType {
	case RawType:
		converter = NewRaw()
	case Ext4Type:
		converter = NewExt4()
	case DiffType:
		converter = NewDiff()
	case RdiffType:
		converter = NewRdiff()
	case GzipType:
		converter = NewGzip()
	case TarGzipType:
		converter = NewTarGzip()
	case XzType:
		converter = NewXz()
	case TarXzType:
		converter = NewTarXz()
	case VhdType:
		const gen2 = false
		converter = NewVhd(gen2)
	case VhdxType:
		const gen2 = true
		converter = NewVhd(gen2)
	case InitrdType:
		converter = NewInitrd()
	case OvaType:
		converter = NewOva()
	default:
		err = fmt.Errorf("unsupported output format: '%s'", formatType)
	}

	return
}
