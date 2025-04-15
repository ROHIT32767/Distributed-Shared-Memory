import pandas as pd

# Load the CSV file
df = pd.read_csv('metrics.csv')

# Filter rows where slave_count == 5
filtered_df = df[df['slave_count'] == 5]

# Drop non-metric columns
columns_to_drop = ['timestamp', 'slave_count']
metric_df = filtered_df.drop(columns=columns_to_drop, errors='ignore')

# Group by client_count and take the first entry for each
first_entry_df = metric_df.groupby('client_count').first()

# Save to a new CSV
first_entry_df.to_csv('first_entry_metrics_by_client_count.csv')
