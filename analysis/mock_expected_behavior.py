import numpy as np
import pandas as pd
import matplotlib.pyplot as plt

# Simulate 10 minutes (600 seconds) of data, 1 tick per second
time = np.arange(0, 600)

# Initialize arrays
heartrate = np.full(600, 70.0)
energy = np.full(600, 100.0)
ma = np.full(600, 0.5) # Model Attention
ne = np.full(600, 0.2) # Negative Emotion
sleep = np.full(600, 0) # 0=Awake, 1=TempSleep

# Phase 1: 0-2 mins (Normal convo)
for t in range(0, 120):
    ma[t] = np.random.uniform(0.4, 0.6)
    heartrate[t] = 70 + np.sin(t / 10) * 5
    energy[t] = 100 - (t * 0.05)

# Phase 2: 2-4 mins (High intensity argument)
for t in range(120, 240):
    ma[t] = np.random.uniform(0.8, 1.0)
    ne[t] = np.random.uniform(0.7, 0.9)
    # Heartrate spikes and stays high
    heartrate[t] = heartrate[t-1] + np.random.uniform(0, 5)
    if heartrate[t] > 130: heartrate[t] = 130
    
    # Energy drains much faster due to high HR and Negative Emotion
    energy[t] = energy[t-1] - 0.25

# Phase 3: 4-7 mins (User goes idle, system cools down)
for t in range(240, 420):
    ma[t] = 0.2
    ne[t] = 0.2
    
    # Heartrate decays back to 70
    heartrate[t] = heartrate[t-1] - 0.5
    if heartrate[t] < 70: heartrate[t] = 70
    
    # Energy stops draining and slowly regenerates
    if heartrate[t] <= 75:
        energy[t] = energy[t-1] + 0.1
    else:
        energy[t] = energy[t-1]
        
    # Sleep mode triggers after 1 min of idle (t=300)
    if t > 300:
        sleep[t] = 1

# Phase 4: 7-10 mins (User returns, normal convo but energy is lower)
for t in range(420, 600):
    ma[t] = np.random.uniform(0.5, 0.7)
    ne[t] = 0.2
    
    # Heartrate spikes again but is capped by lower energy
    energy_factor = energy[t-1] / 100.0
    spike = np.random.uniform(0, 3) * energy_factor
    heartrate[t] = heartrate[t-1] + spike
    if heartrate[t] > (70 + (60 * energy_factor)): 
        heartrate[t] = (70 + (60 * energy_factor))
        
    energy[t] = energy[t-1] - 0.1

df = pd.DataFrame({
    'Time (s)': time,
    'Heartrate': heartrate,
    'Mental Energy': energy,
    'Model Attention': ma,
    'Negative Emotion': ne,
    'Sleep Mode': sleep
})

plt.style.use('dark_background')
fig, (ax1, ax2) = plt.subplots(2, 1, figsize=(12, 8), sharex=True)

# Top Plot: Biological State
ax1.plot(df['Time (s)'], df['Heartrate'], color='#ff4d4d', label='Heartrate (BPM)', linewidth=2)
ax1.plot(df['Time (s)'], df['Mental Energy'], color='#4da6ff', label='Mental Energy (%)', linewidth=2)
ax1.set_ylabel('Value')
ax1.set_title('Expected Subconscious Behavior: Biological State')
ax1.legend(loc='upper right')
ax1.grid(True, alpha=0.3)

# Add event lines to ax1
ax1.axvline(x=240, color='gray', linestyle='--', alpha=0.7)
ax1.text(245, 120, 'User Idle', color='gray', rotation=90)
ax1.axvline(x=300, color='#b366ff', linestyle='--', alpha=0.7)
ax1.text(305, 110, 'Temp Sleep (Consolidate)', color='#b366ff', rotation=90)
ax1.axvline(x=360, color='#ffff66', linestyle='--', alpha=0.7)
ax1.text(365, 110, 'Introspect', color='#ffff66', rotation=90)
ax1.axvline(x=420, color='gray', linestyle='--', alpha=0.7)
ax1.text(425, 120, 'User Returns', color='gray', rotation=90)

# Sleep shading
ax1.fill_between(df['Time (s)'], 0, 140, where=(df['Sleep Mode'] == 1), color='purple', alpha=0.1)


# Bottom Plot: Mindstate
ax2.plot(df['Time (s)'], df['Model Attention'], color='#33ff33', label='Model Attention', linewidth=2)
ax2.plot(df['Time (s)'], df['Negative Emotion'], color='#ff9933', label='Negative Emotion', linewidth=2)
ax2.set_xlabel('Time (seconds)')
ax2.set_ylabel('Score (0.0 - 1.0)')
ax2.set_title('Expected Subconscious Behavior: Mindstate Scores')
ax2.legend(loc='upper right')
ax2.grid(True, alpha=0.3)

# Sleep shading
ax2.fill_between(df['Time (s)'], 0, 1, where=(df['Sleep Mode'] == 1), color='purple', alpha=0.1)

plt.tight_layout()
plt.savefig('/Users/pratheeksha/.gemini/antigravity-ide/brain/6e5e9f08-7843-4ca9-997d-50ee442a840d/expected_behavior.png', dpi=150, bbox_inches='tight')
print("Graph generated at expected_behavior.png")
