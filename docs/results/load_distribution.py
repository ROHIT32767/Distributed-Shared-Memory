import random
import math
import matplotlib.pyplot as plt
from collections import defaultdict

def select_slaves(slaves):
    """Select a subset of slaves using the given formula"""
    num_selected = math.ceil((len(slaves) + 1.0) * 0.5)
    return random.sample(slaves, num_selected)

def simulate_load_distribution(num_slaves=5, num_trials=1000):
    """Simulate slave selection and plot load distribution"""
    slaves = [f"Slave-{i}" for i in range(1, num_slaves+1)]
    selection_counts = defaultdict(int)
    
    # Run simulations
    for _ in range(num_trials):
        selected = select_slaves(slaves)
        for slave in selected:
            selection_counts[slave] += 1
    
    # Prepare data for plotting
    slave_names = sorted(selection_counts.keys())
    counts = [selection_counts[name] for name in slave_names]
    percentages = [count/num_trials*100 for count in counts]
    
    # Create figure
    plt.figure(figsize=(10, 6))
    
    # Bar plot
    bars = plt.bar(slave_names, counts, color='skyblue')
    plt.xlabel('Slave Nodes')
    plt.ylabel('Selection Count')
    plt.title(f'Load Distribution Across {num_slaves} Slaves ({num_trials} trials)')
    
    # Add percentage labels
    for bar, percentage in zip(bars, percentages):
        height = bar.get_height()
        plt.text(bar.get_x() + bar.get_width()/2., height,
                f'{percentage:.1f}%',
                ha='center', va='bottom')
    
    # Add theoretical line
    expected = num_trials * (math.ceil((num_slaves + 1.0) * 0.5)/num_slaves)
    plt.legend()
    plt.grid(axis='y', alpha=0.4)
    plt.tight_layout()
    
    # Save and show
    plt.savefig('load_distribution.png')
    plt.show()

if __name__ == "__main__":
    # Adjust these parameters as needed
    simulate_load_distribution(num_slaves=9, num_trials=10000)