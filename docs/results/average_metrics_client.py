import pandas as pd

# Load the CSV file
df = pd.read_csv('metrics.csv')

# Filter rows where slave_count == 5
filtered_df = df[df['slave_count'] == 5]

# Drop non-metric columns
columns_to_drop = ['timestamp', 'slave_count']
metric_df = filtered_df.drop(columns=columns_to_drop, errors='ignore')

# Group by client_count and calculate mean
grouped_avg = metric_df.groupby('client_count').mean(numeric_only=True)

# Save to a new CSV
grouped_avg.to_csv('average_metrics_by_client_count.csv')
