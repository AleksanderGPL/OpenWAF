package rules

import (
	"fmt"
	"regexp"
	"strings"
)

// builtinDescriptions supplies display text for rules without an upstream msg.
// Descriptions follow the bundled Coraza configuration and CRS 4.25.0; they do
// not alter engine actions or logging. Upstream messages take precedence.
// Unexpanded runtime macros are removed from catalog text because a score only
// exists on a matched transaction.
var builtinDescriptions = map[int]string{
	// @coraza.conf-recommended
	200000: "Select XML request body processor",
	200001: "Select JSON request body processor",
	200006: "Select JSON request body processor for +json content types",

	// REQUEST-901-INITIALIZATION.conf
	901100: "Initialize default inbound anomaly threshold",
	901110: "Initialize default outbound anomaly threshold",
	901111: "Initialize default reporting level",
	901115: "Initialize default early blocking setting",
	901120: "Initialize default blocking paranoia level",
	901125: "Initialize default detection paranoia level",
	901130: "Initialize default sampling percentage",
	901140: "Initialize default critical severity anomaly score",
	901141: "Initialize default error severity anomaly score",
	901142: "Initialize default warning severity anomaly score",
	901143: "Initialize default notice severity anomaly score",
	901160: "Initialize default allowed HTTP methods",
	901162: "Initialize default allowed request content types",
	901163: "Initialize default allowed HTTP versions",
	901164: "Initialize default restricted file extensions",
	901165: "Initialize default restricted headers",
	901167: "Initialize default URL-encoded body processor enforcement setting",
	901168: "Initialize default allowed request charsets",
	901169: "Initialize default UTF-8 validation setting",
	901170: "Initialize default response analysis setting",
	901171: "Initialize default extended restricted headers",
	901200: "Initialize transaction anomaly scores",
	901320: "Initialize global and IP collections using the client User-Agent",
	901400: "Skip sampling selection when all requests are inspected",
	901410: "Calculate request sampling value from the unique request ID",
	901510: "Initialize default method override parameter setting",

	// REQUEST-905-COMMON-EXCEPTIONS.conf
	905100: "Exclude localhost GET / requests from CRS inspection and audit logging",
	905110: "Exclude Apache internal dummy connections from CRS inspection and audit logging",

	// REQUEST-911-METHOD-ENFORCEMENT.conf
	911011: "Skip method enforcement checks below paranoia level 1 (phase 1)",
	911012: "Skip method enforcement checks below paranoia level 1 (phase 2)",
	911013: "Skip method enforcement checks below paranoia level 2 (phase 1)",
	911014: "Skip method enforcement checks below paranoia level 2 (phase 2)",
	911015: "Skip method enforcement checks below paranoia level 3 (phase 1)",
	911016: "Skip method enforcement checks below paranoia level 3 (phase 2)",
	911017: "Skip method enforcement checks below paranoia level 4 (phase 1)",
	911018: "Skip method enforcement checks below paranoia level 4 (phase 2)",

	// REQUEST-913-SCANNER-DETECTION.conf
	913011: "Skip scanner detection checks below paranoia level 1 (phase 1)",
	913012: "Skip scanner detection checks below paranoia level 1 (phase 2)",
	913013: "Skip scanner detection checks below paranoia level 2 (phase 1)",
	913014: "Skip scanner detection checks below paranoia level 2 (phase 2)",
	913015: "Skip scanner detection checks below paranoia level 3 (phase 1)",
	913016: "Skip scanner detection checks below paranoia level 3 (phase 2)",
	913017: "Skip scanner detection checks below paranoia level 4 (phase 1)",
	913018: "Skip scanner detection checks below paranoia level 4 (phase 2)",

	// REQUEST-920-PROTOCOL-ENFORCEMENT.conf
	920011: "Skip protocol enforcement checks below paranoia level 1 (phase 1)",
	920012: "Skip protocol enforcement checks below paranoia level 1 (phase 2)",
	920013: "Skip protocol enforcement checks below paranoia level 2 (phase 1)",
	920014: "Skip protocol enforcement checks below paranoia level 2 (phase 2)",
	920015: "Skip protocol enforcement checks below paranoia level 3 (phase 1)",
	920016: "Skip protocol enforcement checks below paranoia level 3 (phase 2)",
	920017: "Skip protocol enforcement checks below paranoia level 4 (phase 1)",
	920018: "Skip protocol enforcement checks below paranoia level 4 (phase 2)",
	920539: "Skip Unicode bypass check for JSON request bodies",

	// REQUEST-921-PROTOCOL-ATTACK.conf
	921011: "Skip protocol attack checks below paranoia level 1 (phase 1)",
	921012: "Skip protocol attack checks below paranoia level 1 (phase 2)",
	921013: "Skip protocol attack checks below paranoia level 2 (phase 1)",
	921014: "Skip protocol attack checks below paranoia level 2 (phase 2)",
	921015: "Skip protocol attack checks below paranoia level 3 (phase 1)",
	921016: "Skip protocol attack checks below paranoia level 3 (phase 2)",
	921017: "Skip protocol attack checks below paranoia level 4 (phase 1)",
	921018: "Skip protocol attack checks below paranoia level 4 (phase 2)",
	921170: "Count occurrences of each request parameter name",

	// REQUEST-922-MULTIPART-ATTACK.conf
	922140: "Initialize multipart Content-Type header counter",
	922150: "Collect multipart Content-Type headers",

	// REQUEST-930-APPLICATION-ATTACK-LFI.conf
	930011: "Skip local file inclusion checks below paranoia level 1 (phase 1)",
	930012: "Skip local file inclusion checks below paranoia level 1 (phase 2)",
	930013: "Skip local file inclusion checks below paranoia level 2 (phase 1)",
	930014: "Skip local file inclusion checks below paranoia level 2 (phase 2)",
	930015: "Skip local file inclusion checks below paranoia level 3 (phase 1)",
	930016: "Skip local file inclusion checks below paranoia level 3 (phase 2)",
	930017: "Skip local file inclusion checks below paranoia level 4 (phase 1)",
	930018: "Skip local file inclusion checks below paranoia level 4 (phase 2)",

	// REQUEST-931-APPLICATION-ATTACK-RFI.conf
	931011: "Skip remote file inclusion checks below paranoia level 1 (phase 1)",
	931012: "Skip remote file inclusion checks below paranoia level 1 (phase 2)",
	931013: "Skip remote file inclusion checks below paranoia level 2 (phase 1)",
	931014: "Skip remote file inclusion checks below paranoia level 2 (phase 2)",
	931015: "Skip remote file inclusion checks below paranoia level 3 (phase 1)",
	931016: "Skip remote file inclusion checks below paranoia level 3 (phase 2)",
	931017: "Skip remote file inclusion checks below paranoia level 4 (phase 1)",
	931018: "Skip remote file inclusion checks below paranoia level 4 (phase 2)",

	// REQUEST-932-APPLICATION-ATTACK-RCE.conf
	932011: "Skip remote code execution checks below paranoia level 1 (phase 1)",
	932012: "Skip remote code execution checks below paranoia level 1 (phase 2)",
	932013: "Skip remote code execution checks below paranoia level 2 (phase 1)",
	932014: "Skip remote code execution checks below paranoia level 2 (phase 2)",
	932015: "Skip remote code execution checks below paranoia level 3 (phase 1)",
	932016: "Skip remote code execution checks below paranoia level 3 (phase 2)",
	932017: "Skip remote code execution checks below paranoia level 4 (phase 1)",
	932018: "Skip remote code execution checks below paranoia level 4 (phase 2)",

	// REQUEST-933-APPLICATION-ATTACK-PHP.conf
	933011: "Skip PHP injection checks below paranoia level 1 (phase 1)",
	933012: "Skip PHP injection checks below paranoia level 1 (phase 2)",
	933013: "Skip PHP injection checks below paranoia level 2 (phase 1)",
	933014: "Skip PHP injection checks below paranoia level 2 (phase 2)",
	933015: "Skip PHP injection checks below paranoia level 3 (phase 1)",
	933016: "Skip PHP injection checks below paranoia level 3 (phase 2)",
	933017: "Skip PHP injection checks below paranoia level 4 (phase 1)",
	933018: "Skip PHP injection checks below paranoia level 4 (phase 2)",

	// REQUEST-934-APPLICATION-ATTACK-GENERIC.conf
	934011: "Skip generic application attack checks below paranoia level 1 (phase 1)",
	934012: "Skip generic application attack checks below paranoia level 1 (phase 2)",
	934013: "Skip generic application attack checks below paranoia level 2 (phase 1)",
	934014: "Skip generic application attack checks below paranoia level 2 (phase 2)",
	934015: "Skip generic application attack checks below paranoia level 3 (phase 1)",
	934016: "Skip generic application attack checks below paranoia level 3 (phase 2)",
	934017: "Skip generic application attack checks below paranoia level 4 (phase 1)",
	934018: "Skip generic application attack checks below paranoia level 4 (phase 2)",

	// REQUEST-941-APPLICATION-ATTACK-XSS.conf
	941010: "Exclude complex request filenames from selected XSS checks for performance",
	941011: "Skip cross-site scripting checks below paranoia level 1 (phase 1)",
	941012: "Skip cross-site scripting checks below paranoia level 1 (phase 2)",
	941013: "Skip cross-site scripting checks below paranoia level 2 (phase 1)",
	941014: "Skip cross-site scripting checks below paranoia level 2 (phase 2)",
	941015: "Skip cross-site scripting checks below paranoia level 3 (phase 1)",
	941016: "Skip cross-site scripting checks below paranoia level 3 (phase 2)",
	941017: "Skip cross-site scripting checks below paranoia level 4 (phase 1)",
	941018: "Skip cross-site scripting checks below paranoia level 4 (phase 2)",

	// REQUEST-942-APPLICATION-ATTACK-SQLI.conf
	942011: "Skip SQL injection checks below paranoia level 1 (phase 1)",
	942012: "Skip SQL injection checks below paranoia level 1 (phase 2)",
	942013: "Skip SQL injection checks below paranoia level 2 (phase 1)",
	942014: "Skip SQL injection checks below paranoia level 2 (phase 2)",
	942015: "Skip SQL injection checks below paranoia level 3 (phase 1)",
	942016: "Skip SQL injection checks below paranoia level 3 (phase 2)",
	942017: "Skip SQL injection checks below paranoia level 4 (phase 1)",
	942018: "Skip SQL injection checks below paranoia level 4 (phase 2)",

	// REQUEST-943-APPLICATION-ATTACK-SESSION-FIXATION.conf
	943011: "Skip session fixation checks below paranoia level 1 (phase 1)",
	943012: "Skip session fixation checks below paranoia level 1 (phase 2)",
	943013: "Skip session fixation checks below paranoia level 2 (phase 1)",
	943014: "Skip session fixation checks below paranoia level 2 (phase 2)",
	943015: "Skip session fixation checks below paranoia level 3 (phase 1)",
	943016: "Skip session fixation checks below paranoia level 3 (phase 2)",
	943017: "Skip session fixation checks below paranoia level 4 (phase 1)",
	943018: "Skip session fixation checks below paranoia level 4 (phase 2)",

	// REQUEST-944-APPLICATION-ATTACK-JAVA.conf
	944011: "Skip Java attack checks below paranoia level 1 (phase 1)",
	944012: "Skip Java attack checks below paranoia level 1 (phase 2)",
	944013: "Skip Java attack checks below paranoia level 2 (phase 1)",
	944014: "Skip Java attack checks below paranoia level 2 (phase 2)",
	944015: "Skip Java attack checks below paranoia level 3 (phase 1)",
	944016: "Skip Java attack checks below paranoia level 3 (phase 2)",
	944017: "Skip Java attack checks below paranoia level 4 (phase 1)",
	944018: "Skip Java attack checks below paranoia level 4 (phase 2)",

	// REQUEST-949-BLOCKING-EVALUATION.conf
	949011: "Skip inbound blocking evaluation checks below paranoia level 1 (phase 1)",
	949012: "Skip inbound blocking evaluation checks below paranoia level 1 (phase 2)",
	949013: "Skip inbound blocking evaluation checks below paranoia level 2 (phase 1)",
	949014: "Skip inbound blocking evaluation checks below paranoia level 2 (phase 2)",
	949015: "Skip inbound blocking evaluation checks below paranoia level 3 (phase 1)",
	949016: "Skip inbound blocking evaluation checks below paranoia level 3 (phase 2)",
	949017: "Skip inbound blocking evaluation checks below paranoia level 4 (phase 1)",
	949018: "Skip inbound blocking evaluation checks below paranoia level 4 (phase 2)",
	949052: "Add paranoia level 1 scores to the inbound blocking anomaly score (phase 1)",
	949053: "Add paranoia level 2 scores to the inbound blocking anomaly score (phase 1)",
	949054: "Add paranoia level 3 scores to the inbound blocking anomaly score (phase 1)",
	949055: "Add paranoia level 4 scores to the inbound blocking anomaly score (phase 1)",
	949059: "Reset inbound blocking anomaly score before phase 2 evaluation",
	949060: "Add paranoia level 1 scores to the inbound blocking anomaly score (phase 2)",
	949061: "Add paranoia level 2 scores to the inbound blocking anomaly score (phase 2)",
	949062: "Add paranoia level 3 scores to the inbound blocking anomaly score (phase 2)",
	949063: "Add paranoia level 4 scores to the inbound blocking anomaly score (phase 2)",
	949152: "Add paranoia level 1 scores to the inbound detection anomaly score (phase 1)",
	949153: "Add paranoia level 2 scores to the inbound detection anomaly score (phase 1)",
	949154: "Add paranoia level 3 scores to the inbound detection anomaly score (phase 1)",
	949155: "Add paranoia level 4 scores to the inbound detection anomaly score (phase 1)",
	949159: "Reset inbound detection anomaly score before phase 2 evaluation",
	949160: "Add paranoia level 1 scores to the inbound detection anomaly score (phase 2)",
	949161: "Add paranoia level 2 scores to the inbound detection anomaly score (phase 2)",
	949162: "Add paranoia level 3 scores to the inbound detection anomaly score (phase 2)",
	949163: "Add paranoia level 4 scores to the inbound detection anomaly score (phase 2)",

	// RESPONSE-950-DATA-LEAKAGES.conf
	950010: "Skip data leakage checks for compressed responses",
	950011: "Skip data leakage checks below paranoia level 1 (phase 3)",
	950012: "Skip data leakage checks below paranoia level 1 (phase 4)",
	950013: "Skip data leakage checks below paranoia level 2 (phase 3)",
	950014: "Skip data leakage checks below paranoia level 2 (phase 4)",
	950015: "Skip data leakage checks below paranoia level 3 (phase 3)",
	950016: "Skip data leakage checks below paranoia level 3 (phase 4)",
	950017: "Skip data leakage checks below paranoia level 4 (phase 3)",
	950018: "Skip data leakage checks below paranoia level 4 (phase 4)",
	950021: "Skip response inspection when configured",

	// RESPONSE-951-DATA-LEAKAGES-SQL.conf
	951010: "Skip SQL data leakage checks for compressed responses",
	951011: "Skip SQL data leakage checks below paranoia level 1 (phase 3)",
	951012: "Skip SQL data leakage checks below paranoia level 1 (phase 4)",
	951013: "Skip SQL data leakage checks below paranoia level 2 (phase 3)",
	951014: "Skip SQL data leakage checks below paranoia level 2 (phase 4)",
	951015: "Skip SQL data leakage checks below paranoia level 3 (phase 3)",
	951016: "Skip SQL data leakage checks below paranoia level 3 (phase 4)",
	951017: "Skip SQL data leakage checks below paranoia level 4 (phase 3)",
	951018: "Skip SQL data leakage checks below paranoia level 4 (phase 4)",
	951100: "Skip detailed SQL error checks when no SQL error patterns are found",

	// RESPONSE-952-DATA-LEAKAGES-JAVA.conf
	952010: "Skip Java data leakage checks for compressed responses",
	952011: "Skip Java data leakage checks below paranoia level 1 (phase 3)",
	952012: "Skip Java data leakage checks below paranoia level 1 (phase 4)",
	952013: "Skip Java data leakage checks below paranoia level 2 (phase 3)",
	952014: "Skip Java data leakage checks below paranoia level 2 (phase 4)",
	952015: "Skip Java data leakage checks below paranoia level 3 (phase 3)",
	952016: "Skip Java data leakage checks below paranoia level 3 (phase 4)",
	952017: "Skip Java data leakage checks below paranoia level 4 (phase 3)",
	952018: "Skip Java data leakage checks below paranoia level 4 (phase 4)",

	// RESPONSE-953-DATA-LEAKAGES-PHP.conf
	953010: "Skip PHP data leakage checks for compressed responses",
	953011: "Skip PHP data leakage checks below paranoia level 1 (phase 3)",
	953012: "Skip PHP data leakage checks below paranoia level 1 (phase 4)",
	953013: "Skip PHP data leakage checks below paranoia level 2 (phase 3)",
	953014: "Skip PHP data leakage checks below paranoia level 2 (phase 4)",
	953015: "Skip PHP data leakage checks below paranoia level 3 (phase 3)",
	953016: "Skip PHP data leakage checks below paranoia level 3 (phase 4)",
	953017: "Skip PHP data leakage checks below paranoia level 4 (phase 3)",
	953018: "Skip PHP data leakage checks below paranoia level 4 (phase 4)",

	// RESPONSE-954-DATA-LEAKAGES-IIS.conf
	954010: "Skip IIS data leakage checks for compressed responses",
	954011: "Skip IIS data leakage checks below paranoia level 1 (phase 3)",
	954012: "Skip IIS data leakage checks below paranoia level 1 (phase 4)",
	954013: "Skip IIS data leakage checks below paranoia level 2 (phase 3)",
	954014: "Skip IIS data leakage checks below paranoia level 2 (phase 4)",
	954015: "Skip IIS data leakage checks below paranoia level 3 (phase 3)",
	954016: "Skip IIS data leakage checks below paranoia level 3 (phase 4)",
	954017: "Skip IIS data leakage checks below paranoia level 4 (phase 3)",
	954018: "Skip IIS data leakage checks below paranoia level 4 (phase 4)",

	// RESPONSE-955-WEB-SHELLS.conf
	955010: "Skip web shell checks for compressed responses",
	955011: "Skip web shell checks below paranoia level 1 (phase 3)",
	955012: "Skip web shell checks below paranoia level 1 (phase 4)",
	955013: "Skip web shell checks below paranoia level 2 (phase 3)",
	955014: "Skip web shell checks below paranoia level 2 (phase 4)",
	955015: "Skip web shell checks below paranoia level 3 (phase 3)",
	955016: "Skip web shell checks below paranoia level 3 (phase 4)",
	955017: "Skip web shell checks below paranoia level 4 (phase 3)",
	955018: "Skip web shell checks below paranoia level 4 (phase 4)",

	// RESPONSE-956-DATA-LEAKAGES-RUBY.conf
	956010: "Skip Ruby data leakage checks for compressed responses",
	956011: "Skip Ruby data leakage checks below paranoia level 1 (phase 3)",
	956012: "Skip Ruby data leakage checks below paranoia level 1 (phase 4)",
	956013: "Skip Ruby data leakage checks below paranoia level 2 (phase 3)",
	956014: "Skip Ruby data leakage checks below paranoia level 2 (phase 4)",
	956015: "Skip Ruby data leakage checks below paranoia level 3 (phase 3)",
	956016: "Skip Ruby data leakage checks below paranoia level 3 (phase 4)",
	956017: "Skip Ruby data leakage checks below paranoia level 4 (phase 3)",
	956018: "Skip Ruby data leakage checks below paranoia level 4 (phase 4)",

	// RESPONSE-959-BLOCKING-EVALUATION.conf
	959011: "Skip outbound blocking evaluation checks below paranoia level 1 (phase 3)",
	959012: "Skip outbound blocking evaluation checks below paranoia level 1 (phase 4)",
	959013: "Skip outbound blocking evaluation checks below paranoia level 2 (phase 3)",
	959014: "Skip outbound blocking evaluation checks below paranoia level 2 (phase 4)",
	959015: "Skip outbound blocking evaluation checks below paranoia level 3 (phase 3)",
	959016: "Skip outbound blocking evaluation checks below paranoia level 3 (phase 4)",
	959017: "Skip outbound blocking evaluation checks below paranoia level 4 (phase 3)",
	959018: "Skip outbound blocking evaluation checks below paranoia level 4 (phase 4)",
	959052: "Add paranoia level 1 scores to the outbound blocking anomaly score (phase 3)",
	959053: "Add paranoia level 2 scores to the outbound blocking anomaly score (phase 3)",
	959054: "Add paranoia level 3 scores to the outbound blocking anomaly score (phase 3)",
	959055: "Add paranoia level 4 scores to the outbound blocking anomaly score (phase 3)",
	959059: "Reset outbound blocking anomaly score before phase 4 evaluation",
	959060: "Add paranoia level 1 scores to the outbound blocking anomaly score (phase 4)",
	959061: "Add paranoia level 2 scores to the outbound blocking anomaly score (phase 4)",
	959062: "Add paranoia level 3 scores to the outbound blocking anomaly score (phase 4)",
	959063: "Add paranoia level 4 scores to the outbound blocking anomaly score (phase 4)",
	959152: "Add paranoia level 1 scores to the outbound detection anomaly score (phase 3)",
	959153: "Add paranoia level 2 scores to the outbound detection anomaly score (phase 3)",
	959154: "Add paranoia level 3 scores to the outbound detection anomaly score (phase 3)",
	959155: "Add paranoia level 4 scores to the outbound detection anomaly score (phase 3)",
	959159: "Reset outbound detection anomaly score before phase 4 evaluation",
	959160: "Add paranoia level 1 scores to the outbound detection anomaly score (phase 4)",
	959161: "Add paranoia level 2 scores to the outbound detection anomaly score (phase 4)",
	959162: "Add paranoia level 3 scores to the outbound detection anomaly score (phase 4)",
	959163: "Add paranoia level 4 scores to the outbound detection anomaly score (phase 4)",

	// RESPONSE-980-CORRELATION.conf
	980011: "Skip anomaly correlation checks below paranoia level 1 (phase 1)",
	980012: "Skip anomaly correlation checks below paranoia level 1 (phase 2)",
	980013: "Skip anomaly correlation checks below paranoia level 2 (phase 1)",
	980014: "Skip anomaly correlation checks below paranoia level 2 (phase 2)",
	980015: "Skip anomaly correlation checks below paranoia level 3 (phase 1)",
	980016: "Skip anomaly correlation checks below paranoia level 3 (phase 2)",
	980017: "Skip anomaly correlation checks below paranoia level 4 (phase 1)",
	980018: "Skip anomaly correlation checks below paranoia level 4 (phase 2)",
	980041: "Skip anomaly reporting when reporting is disabled",
	980042: "Always report anomaly scores at reporting level 5 or higher",
	980043: "Skip anomaly reporting when the detection anomaly score is zero",
	980044: "Report anomalies when the inbound blocking threshold is reached",
	980045: "Report anomalies when the outbound blocking threshold is reached",
	980046: "Skip further anomaly reporting below reporting level 2",
	980047: "Report anomalies when the inbound detection threshold is reached",
	980048: "Report anomalies when the outbound detection threshold is reached",
	980049: "Skip further anomaly reporting below reporting level 3",
	980050: "Report anomalies when the blocking anomaly score is positive",
	980051: "Skip further anomaly reporting below reporting level 4",
	980099: "Combine inbound and outbound anomaly scores for reporting",
}

var totalScoreMacro = regexp.MustCompile(`\s*\(Total Score: %\{[^}]+\}\)`)
var messageMacro = regexp.MustCompile(`%\{(?:TX\.)?([^}]+)\}`)

func catalogMessage(id int, source, upstream string) string {
	if upstream != "" {
		return displayMessage(upstream)
	}
	if description := builtinDescriptions[id]; description != "" {
		return description
	}
	return fmt.Sprintf("Internal %s rule %d", strings.ToUpper(source), id)
}

func displayMessage(msg string) string {
	if !strings.Contains(msg, "%{") {
		return msg
	}
	msg = totalScoreMacro.ReplaceAllString(msg, "")
	msg = messageMacro.ReplaceAllStringFunc(msg, func(token string) string {
		name := messageMacro.FindStringSubmatch(token)[1]
		return strings.ReplaceAll(strings.ToLower(name), "_", " ")
	})
	return strings.TrimSpace(msg)
}
