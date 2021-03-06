package generator

import (
	"bytes"
	"fmt"
	"github.com/fatih/camelcase"
	"github.com/pkg/errors"
	"sort"
	"strings"
	"text/template"
)

func printStats(acts actions, policies []*policyFile) ([]byte, error) {
	tmpl, err := template.New("").Parse(`# AWS IAM Tracker

This project collects IAM actions, AWS APIs and managed policies from various public sources.

You can explore the data collected using [the static site](https://glassechidna.github.io/trackiam/).

Collected data is published to the [policies](/policies) and [services](/services) folders in this repo.

Thank you to [alanakirby/aktion](https://github.com/alanakirby/aktion) for originally 
having this idea and being gracious about me shamelessly ripping it off.
	
# Stats

* Unique services: {{ .ServiceCount }}
* Unique actions: {{ .ActionCount }}
* Managed policies: {{ .PolicyCount }}

Most common managed policy name prefixes:

| Policy ARN | Count |
| ------ | ----- |
{{- range .PolicyPrefixes }}
| {{ $.Backtick }}arn:aws:iam::aws:policy/{{ .Prefix }}*{{ $.Backtick }} | {{ .Count }} |
{{- end }}
| Other | {{ .OtherPolicyPrefixCount }} |

The following table summarises the AWS APIs. 

* The first column is the name of the API as far as IAM policies are concerned. 
* The second column is IAM actions that exactly match the names of invokable 
  APIs exposed by AWS.
* The third column is invokable APIs that don't have a corresponding IAM action.
* The fourth column is IAM actions that don't have a corresponding invokable API.

| Service | Action/API pairs | APIs without actions | Actions without APIs |
| ------ | ----- | ----- | ----- |
{{- range .Services }}
| [{{ $.Backtick }}{{ .IamPrefix }}{{ $.Backtick }}](services/{{ .IamPrefix }}.yml) | {{ .ActionApiPairs }} | {{ .ActionlessApis }} | {{ .ApilessActions }} |
{{- end }}

Most common action prefixes:

| Prefix | Count |
| ------ | ----- |
{{- range .Prefixes }}
| {{ $.Backtick }}{{ .Prefix }}{{ $.Backtick }} | {{ .Count }} |
{{- end }}

`)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	policyPrefixes, otherPolicyPrefixCount := policyPrefixStats(policies)

	byIamPrefix := acts.groupBy(func(act *action) string {
		return act.IamPrefix
	})

	stats := []serviceStats{}

	for iamPrefix, actz := range byIamPrefix {
		bySupport := actz.groupBy(func(act *action) string {
			return fmt.Sprintf("%v%v", act.HasApi, act.HasAction)
		})

		hasApiAndAction := bySupport["truetrue"]
		hasActionWithoutApi := bySupport["falsetrue"]
		hasApiWithoutAction := bySupport["truefalse"]

		stats = append(stats, serviceStats{
			IamPrefix:      iamPrefix,
			ActionApiPairs: len(hasApiAndAction),
			ActionlessApis: len(hasApiWithoutAction),
			ApilessActions: len(hasActionWithoutApi),
		})
	}

	sort.Sort(sort.Reverse(serviceStatsByCount(stats)))

	byFirstWord := acts.groupBy(func(act *action) string {
		words := camelcase.Split(act.Name)
		return words[0]
	})

	wordStats := []stat{}
	for word, acts := range byFirstWord {
		wordStats = append(wordStats, stat{Prefix: word, Count: len(acts)})
	}
	sort.Sort(sort.Reverse(statsByCount(wordStats)))
	wordStats = wordStats[:10]

	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, map[string]interface{}{
		"ServiceCount":           len(stats),
		"ActionCount":            len(acts),
		"PolicyCount":            len(policies),
		"Prefixes":               wordStats,
		"PolicyPrefixes":         policyPrefixes,
		"OtherPolicyPrefixCount": otherPolicyPrefixCount,
		"Services":               stats,
		"Backtick":               "`",
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return buf.Bytes(), nil
}

func policyPrefixStats(files []*policyFile) ([]stat, int) {
	prefixes := []string{
		"AWS",
		"Amazon",
		"aws-service-role/",
		"job-function/",
		"service-role/",
	}

	countMap := map[string]int{}
	other := 0

OUTER:
	for _, file := range files {
		bits := strings.Split(file.Arn, ":")
		resource := strings.TrimPrefix(bits[len(bits)-1], "policy/")
		for _, prefix := range prefixes {
			if strings.HasPrefix(resource, prefix) {
				countMap[prefix] += 1
				continue OUTER
			}
		}

		other++
	}

	stats := []stat{}
	for prefix, count := range countMap {
		stats = append(stats, stat{Prefix: prefix, Count: count})
	}
	sort.Sort(sort.Reverse(statsByCount(stats)))

	return stats, other
}

type stat struct {
	Prefix string
	Count  int
}

type statsByCount []stat

func (s statsByCount) Len() int {
	return len(s)
}
func (s statsByCount) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s statsByCount) Less(i, j int) bool {
	if s[i].Count != s[j].Count {
		return s[i].Count < s[j].Count
	} else {
		return s[i].Prefix < s[j].Prefix
	}
}

type serviceStats struct {
	IamPrefix      string
	ActionApiPairs int
	ActionlessApis int
	ApilessActions int
}

type serviceStatsByCount []serviceStats

func (s serviceStatsByCount) Len() int {
	return len(s)
}
func (s serviceStatsByCount) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s serviceStatsByCount) Less(i, j int) bool {
	if s[i].ActionApiPairs != s[j].ActionApiPairs {
		return s[i].ActionApiPairs < s[j].ActionApiPairs
	} else if s[i].ActionlessApis != s[j].ActionlessApis {
		return s[i].ActionlessApis < s[j].ActionlessApis
	} else if s[i].ApilessActions != s[j].ApilessActions {
		return s[i].ApilessActions < s[j].ApilessActions
	} else {
		return s[i].IamPrefix < s[j].IamPrefix
	}
}
