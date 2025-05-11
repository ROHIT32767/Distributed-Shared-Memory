import pandas as pd

# Load the CSV file
df = pd.read_csv('metrics.csv')

# Filter rows where client_count == 20
filtered_df = df[df['client_count'] == 20]

# Drop non-metric columns
columns_to_drop = ['timestamp', 'client_count']
metric_df = filtered_df.drop(columns=columns_to_drop, errors='ignore')

# Group by slave_count and calculate the mean for each group
grouped_avg = metric_df.groupby('slave_count').mean(numeric_only=True)

# Save the result to a new CSV
grouped_avg.to_csv('average_metrics_by_slave_count.csv')
