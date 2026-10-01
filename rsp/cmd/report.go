package cmd

import "github.com/spf13/cobra"

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.AddCommand(miningReportCmd)
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate reports entirely from the cache",
}

var travelReportCmd = &cobra.Command{
	Use:   "travel",
	Short: "Display information about devices in motion",
	RunE: func(cmd *cobra.Command, args []string) error {
		// select array_agg(code), dest from (
		//     select code, split_part(data->'travel'->>'destination', '-', 1) dest
		//     from json_devices
		//     where data->'travel'->'destination' is not null
		// ) group by dest

		return nil
	},
}

var miningReportCmd = &cobra.Command{
	Use:   "mining",
	Short: "Show mining rates",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Wild query from AG to get the full report
		rows, err := db.Query(`
		  WITH snapshots AS (
			SELECT
			  location,
			  created,
			  (data->'report'->>'total_resources_available')::numeric AS total,
			  LAG(created) OVER (PARTITION BY location ORDER BY created) AS prev_created,
			  LAG((data->'report'->>'total_resources_available')::numeric) OVER (PARTITION BY location ORDER BY created) AS prev_total
			FROM event_stream
			WHERE event = 'ami.survey.digest'
		  ),
		  declines AS (
			SELECT
			  location,
			  prev_total - total AS removed,
			  EXTRACT(EPOCH FROM (created - prev_created)) AS seconds
			FROM snapshots
			-- Isolate actual mining intervals (ignoring downtime at 0 and additions):
			WHERE prev_total > total
			  AND prev_total > 0
			  AND EXTRACT(EPOCH FROM (created - prev_created)) > 0
		  ),
		  rates AS (
			SELECT
			  location,
			  SUM(removed) AS total_resources_removed,
			  ROUND(SUM(seconds)::numeric / 3600, 2) AS total_mining_hours,
			  ROUND((SUM(removed) / (SUM(seconds) / 60.0))::numeric, 2) AS avg_decline_per_min,
			  ROUND((SUM(removed) / (SUM(seconds) / 3600.0))::numeric, 2) AS avg_decline_per_hour
			FROM declines
			GROUP BY location
		  ),
		  latest_mining_digests AS (
			-- Grab the most recent mining digest for each belt
			SELECT DISTINCT ON (location)
			  location,
			  data->'devices' AS devices
			FROM event_stream
			WHERE event = 'ami.mining.digest'
			ORDER BY location, created DESC
		  ),
		  drone_summary AS (
			-- Expand the devices array and look up device types in json_devices
			SELECT
			  dt.location,
			  STRING_AGG(format('%s %s', dt.count, dt.type), ', ' ORDER BY dt.type) AS drone_breakdown
			FROM (
			  SELECT
				lmd.location,
				COALESCE(jd.type, 'unknown') AS type,
				COUNT(*) AS count
			  FROM latest_mining_digests lmd,
			  LATERAL jsonb_array_elements(lmd.devices) AS dev
			  LEFT JOIN json_devices jd ON jd.code = dev->>'device_code'
			  GROUP BY lmd.location, jd.type
			) dt
			GROUP BY dt.location
		  )
		  SELECT
			b.designation AS belt,
			b.density,
			COALESCE(ds.drone_breakdown, 'none') AS drone_types,
			r.total_resources_removed,
			r.total_mining_hours,
			r.avg_decline_per_min,
			r.avg_decline_per_hour
		  FROM rates r
		  JOIN belts b ON b.designation = r.location
		  LEFT JOIN drone_summary ds ON ds.location = r.location
		  ORDER BY r.avg_decline_per_hour DESC`)
		if err != nil {
			return err
		}
		defer rows.Close()
		var data [][]any
		for rows.Next() {
			var (
				belt, density, types       string
				total_mined                int
				mining_hours, per_h, per_m float32
			)
			if err := rows.Scan(&belt, &density, &types, &total_mined, &mining_hours, &per_m, &per_h); err != nil {
				return err
			}
			data = append(data, []any{
				belt, density, types, total_mined, mining_hours, per_m, per_h,
			})
		}
		printTable([]string{"Belt", "Density", "Drones", "Totals", "Mining hours", "Per minute", "Per hour"}, data)

		return nil
	},
}
