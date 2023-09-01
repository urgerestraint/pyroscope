package jfr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	phlaremodel "github.com/grafana/pyroscope/pkg/model"
	"github.com/grafana/pyroscope/pkg/og/convert/pprof/bench"
	"github.com/grafana/pyroscope/pkg/og/storage"
	"github.com/grafana/pyroscope/pkg/og/storage/segment"
	"github.com/grafana/pyroscope/pkg/pprof"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
)

func TestParser(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Java JFR Parser suite")
}

func TestParseCompareExpectedData(t *testing.T) {
	testdata := []struct {
		jfr    string
		labels string
	}{
		{"testdata/cortex-dev-01__kafka-0__cpu__0.jfr.gz", ""},
		{"testdata/cortex-dev-01__kafka-0__cpu__1.jfr.gz", ""},
		{"testdata/cortex-dev-01__kafka-0__cpu__2.jfr.gz", ""},
		{"testdata/cortex-dev-01__kafka-0__cpu__3.jfr.gz", ""},
		{"testdata/cortex-dev-01__kafka-0__cpu_lock_alloc__0.jfr.gz", ""},
		{"testdata/cortex-dev-01__kafka-0__cpu_lock_alloc__1.jfr.gz", ""},
		{"testdata/cortex-dev-01__kafka-0__cpu_lock_alloc__2.jfr.gz", ""},
		{"testdata/cortex-dev-01__kafka-0__cpu_lock_alloc__3.jfr.gz", ""},
		{"testdata/cortex-dev-01__kafka-0__cpu_lock0_alloc0__0.jfr.gz", ""},
		{"testdata/dump1.jfr.gz", "testdata/dump1.labels.pb.gz"},
	}
	for _, td := range testdata {
		t.Run(td.jfr, func(t *testing.T) {
			jfr, err := bench.ReadGzipFile(td.jfr)
			require.NoError(t, err)
			//putter := &bench.MockPutter{Keep: true}
			k, err := segment.ParseKey("kafka.app")
			require.NoError(t, err)

			pi := &storage.PutInput{
				StartTime:  time.UnixMilli(1000),
				EndTime:    time.UnixMilli(2000),
				Key:        k,
				SpyName:    "java",
				SampleRate: 100,
			}
			var labels = new(LabelsSnapshot)
			if td.labels != "" {
				labelsBytes, err := bench.ReadGzipFile(td.labels)
				require.NoError(t, err)
				err = proto.Unmarshal(labelsBytes, labels)
				require.NoError(t, err)
			}
			profiles, err := ParseJFR(context.TODO(), jfr, pi, labels)
			require.NoError(t, err)
			if len(profiles) == 0 {
				t.Fatal(err)
			}
			//todo
			jsonFile := strings.TrimSuffix(td.jfr, ".jfr.gz") + ".json.gz"
			//err = putter.DumpJson(jsonFile)
			err = compareWithJson(profiles, jsonFile)
			require.NoError(t, err)

		})
	}
}

func compareWithJson(profiles []ParsedProfiles, file string) error {
	goldBS, err := bench.ReadGzipFile(file)
	if err != nil {
		return err
	}
	trees := make(map[string]string)
	err = json.Unmarshal(goldBS, &trees)
	if err != nil {
		return err
	}
	for _, profile := range profiles {

		var keys []string
		var valueIndices []int
		ls := phlaremodel.Labels(profile.Labels)
		metric := ls.Get(model.MetricNameLabel)
		service_name := ls.Get("service_name")
		typ := profile.Profile.StringTable[profile.Profile.SampleType[0].Type]

		switch metric {
		case "process_cpu":
			keys = []string{service_name + "." + "cpu{}"}
			valueIndices = []int{0}
		case "memory":

			if strings.Contains(typ, "alloc_in_new_tlab_objects") {
				keys = []string{
					service_name + "." + "alloc_in_new_tlab_objects{}",
					service_name + "." + "alloc_in_new_tlab_bytes{}",
				}
			} else {
				keys = []string{
					service_name + "." + "alloc_outside_tlab_objects{}",
					service_name + "." + "alloc_outside_tlab_bytes{}",
				}
			}
			valueIndices = []int{0, 1}
		case "mutex":
			keys = []string{
				service_name + "." + "lock_count{}",
				service_name + "." + "lock_duration{}",
			}
			valueIndices = []int{0, 1}
		default:
			panic("unknown metric: " + metric + " " + service_name)
		}
		for i := range keys {
			key := keys[i]
			expectedTree := trees[key]
			if expectedTree == "" {
				return fmt.Errorf("no tree found for %s", key)
			}
			expectedLines := strings.Split(expectedTree, "\n")
			slices.Sort(expectedLines)
			expectedTree = strings.Join(expectedLines, "\n")
			expectedTree = strings.Trim(expectedTree, "\n")

			pp := pprof.Profile{Profile: profile.Profile}
			pp.Normalize()
			collapseLines := bench.StackCollapseProto(pp.Profile, valueIndices[i])
			slices.Sort(collapseLines)
			collapsedStr := strings.Join(collapseLines, "\n")
			collapsedStr = strings.Trim(collapsedStr, "\n")

			if expectedTree != collapsedStr {
				os.WriteFile(file+"_expected.txt", []byte(expectedTree), 0644)
				os.WriteFile(file+"_actual.txt", []byte(collapsedStr), 0644)
				return fmt.Errorf("expected tree:\n%s\ngot:\n%s", expectedTree, collapsedStr)
			}
		}

	}
	return nil
}

func BenchmarkParser(b *testing.B) {
	tests := []string{
		"testdata/cortex-dev-01__kafka-0__cpu__0.jfr.gz",
		"testdata/cortex-dev-01__kafka-0__cpu__1.jfr.gz",
		"testdata/cortex-dev-01__kafka-0__cpu__2.jfr.gz",
		"testdata/cortex-dev-01__kafka-0__cpu__3.jfr.gz",
		"testdata/cortex-dev-01__kafka-0__cpu_lock_alloc__0.jfr.gz",
		"testdata/cortex-dev-01__kafka-0__cpu_lock_alloc__1.jfr.gz",
		"testdata/cortex-dev-01__kafka-0__cpu_lock_alloc__2.jfr.gz",
		"testdata/cortex-dev-01__kafka-0__cpu_lock_alloc__3.jfr.gz",
		"testdata/cortex-dev-01__kafka-0__cpu_lock0_alloc0__0.jfr.gz",
	}

	for _, testdata := range tests {
		f := testdata
		b.Run(testdata, func(b *testing.B) {
			jfr, err := bench.ReadGzipFile(f)
			require.NoError(b, err)
			k, err := segment.ParseKey("kafka.app")
			require.NoError(b, err)
			pi := &storage.PutInput{
				StartTime:  time.UnixMilli(1000),
				EndTime:    time.UnixMilli(2000),
				Key:        k,
				SpyName:    "java",
				SampleRate: 100,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				profiles, err := ParseJFR(context.TODO(), jfr, pi, nil)
				if err != nil {
					b.Fatal(err)
				}
				if len(profiles) == 0 {
					b.Fatal()
				}
			}
		})
	}
}
