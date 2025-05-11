import pandas as pd
import matplotlib.pyplot as plt
import os
import sys

def main():
    if len(sys.argv) < 2:
        print("Usage: python visualize.py <results_dir>")
        return
    
    results_dir = sys.argv[1]
    csv_path = os.path.join(results_dir, "metrics.csv")
    
    try:
        df = pd.read_csv(csv_path)
    except Exception as e:
        print(f"Error reading CSV: {e}")
        return
    
    plt.figure(figsize=(15, 10))
    
    # Throughput vs Clients
    plt.subplot(2, 2, 1)
    plt.plot(df['client_count'], df['throughput_rps'], 'b-o')
    plt.title('Throughput vs Number of Clients')
    plt.xlabel('Number of Clients')
    plt.ylabel('Requests per Second')
    plt.grid(True)
    
    # Latency vs Clients
    plt.subplot(2, 2, 2)
    plt.plot(df['client_count'], df['read_latency_us'], 'g-o', label='Read Latency')
    plt.plot(df['client_count'], df['write_latency_us'], 'r-o', label='Write Latency')
    plt.title('Latency vs Number of Clients')
    plt.xlabel('Number of Clients')
    plt.ylabel('Latency (μs)')
    plt.legend()
    plt.grid(True)
    
    # Error Rate vs Clients
    plt.subplot(2, 2, 3)
    error_rate = (df['error_count'] / df['total_requests']) * 100
    plt.plot(df['client_count'], error_rate, 'm-o')
    plt.title('Error Rate vs Number of Clients')
    plt.xlabel('Number of Clients')
    plt.ylabel('Error Rate (%)')
    plt.grid(True)
    
    # Operation Distribution
    plt.subplot(2, 2, 4)
    plt.bar(df['client_count'], df['read_count'], width=3, label='Reads')
    plt.bar(df['client_count'], df['write_count'], width=3, bottom=df['read_count'], label='Writes')
    plt.title('Operation Distribution')
    plt.xlabel('Number of Clients')
    plt.ylabel('Operation Count')
    plt.legend()
    plt.grid(True)
    
    plt.tight_layout()
    plot_path = os.path.join(results_dir, 'performance_metrics.png')
    plt.savefig(plot_path)
    print(f"Saved performance metrics plot to {plot_path}")

if __name__ == "__main__":
    main()