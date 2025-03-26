/*
 *  Copyright 2016-2025 Michael Zillgith
 *
 *  This file is part of lib60870.NET
 *
 *  lib60870.NET is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  lib60870.NET is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with lib60870.NET.  If not, see <http://
 *  
 *  
 *  
 *  www.gnu.org/licenses/>.
 *
 *  See COPYING file for the complete license text.
 */

using lib60870.CS101;
using lib60870.CS104;
using System;
using System.Threading;

namespace lib60870
{
    public class ASDUQueue
    {

        private enum QueueEntryState
        {
            NOT_USED,
            WAITING_FOR_TRANSMISSION,
            SENT_BUT_NOT_CONFIRMED
        }

        private struct ASDUQueueEntry
        {
            public long entryTimestamp;
            public BufferFrame asdu;
            public QueueEntryState state;
        }

        private ASDUQueueEntry[] enqueuedASDUs = null;
        private int oldestQueueEntry = -1; /* first entry in FIFO */
        private int latestQueueEntry = -1; /* last entry in FIFO */
        private int numberOfAsduInQueue = 0; /* number of messages (ASDU) in the queue */
        public SemaphoreSlim queueLock = new SemaphoreSlim(1, 1);
        private int maxQueueSize;

        private EnqueueMode enqueueMode;

        private ApplicationLayerParameters parameters;

        private Action<string> DebugLog = null;

        public int NumberOfAsduInQueue { get => numberOfAsduInQueue; set => numberOfAsduInQueue = value; }

        public ASDUQueue(int maxQueueSize, EnqueueMode enqueueMode, ApplicationLayerParameters parameters, Action<string> DebugLog)
        {
            enqueuedASDUs = new ASDUQueueEntry[maxQueueSize];

            for (int i = 0; i < maxQueueSize; i++)
            {
                enqueuedASDUs[i].asdu = new BufferFrame(new byte[260], 6);
                enqueuedASDUs[i].state = QueueEntryState.NOT_USED;
            }

            this.enqueueMode = enqueueMode;
            oldestQueueEntry = -1;
            latestQueueEntry = -1;
            NumberOfAsduInQueue = 0;
            this.maxQueueSize = maxQueueSize;
            this.parameters = parameters;
            this.DebugLog = DebugLog;
        }

        public void EnqueueAsdu(ASDU asdu)
        {
            lock (enqueuedASDUs)
            {
                if (NumberOfAsduInQueue == 0)
                {
                    oldestQueueEntry = 0;
                    latestQueueEntry = 0;
                    NumberOfAsduInQueue = 1;

                    enqueuedASDUs[0].asdu.ResetFrame();
                    asdu.Encode(enqueuedASDUs[0].asdu, parameters);

                    enqueuedASDUs[0].entryTimestamp = SystemUtils.currentTimeMillis();
                    enqueuedASDUs[0].state = QueueEntryState.WAITING_FOR_TRANSMISSION;
                }
                else
                {
                    bool enqueue = true;

                    if (NumberOfAsduInQueue == maxQueueSize)
                    {
                        if (enqueueMode == EnqueueMode.REMOVE_OLDEST)
                        {
                        }
                        else if (enqueueMode == EnqueueMode.IGNORE)
                        {
                            DebugLog("Queue is full. Ignore new ASDU.");
                            enqueue = false;
                        }
                        else if (enqueueMode == EnqueueMode.THROW_EXCEPTION)
                        {
                            throw new ASDUQueueException("Event queue is full.");
                        }
                    }

                    if (enqueue)
                    {
                        latestQueueEntry = (latestQueueEntry + 1) % maxQueueSize;

                        if (latestQueueEntry == oldestQueueEntry)
                            oldestQueueEntry = (oldestQueueEntry + 1) % maxQueueSize;
                        else
                            NumberOfAsduInQueue++;

                        enqueuedASDUs[latestQueueEntry].asdu.ResetFrame();
                        asdu.Encode(enqueuedASDUs[latestQueueEntry].asdu, parameters);
                        enqueuedASDUs[latestQueueEntry].entryTimestamp = SystemUtils.currentTimeMillis();
                        enqueuedASDUs[latestQueueEntry].state = QueueEntryState.WAITING_FOR_TRANSMISSION;


                    }

                }

            }

            DebugLog("Queue contains " + NumberOfAsduInQueue + " messages (oldest: " + oldestQueueEntry + " latest: " + latestQueueEntry + ")");

        }

        public void LockASDUQueue()
        {
            Monitor.Enter(enqueuedASDUs);
        }

        public void UnlockASDUQueue()
        {
            Monitor.Exit(enqueuedASDUs);
        }

        public bool MessageQueue_hasUnconfirmedIMessages()
        {
            bool retVal = false;

            if (NumberOfAsduInQueue != 0)
            {
                int currentIndex = oldestQueueEntry;

                while (currentIndex > 0)
                {
                    if (enqueuedASDUs[currentIndex].state == QueueEntryState.SENT_BUT_NOT_CONFIRMED)
                    {
                        retVal = true;
                        break;
                    }

                    if (currentIndex == latestQueueEntry)
                        break;

                    if (enqueuedASDUs[currentIndex].state == QueueEntryState.NOT_USED)
                        break;

                    currentIndex = (currentIndex + 1) % maxQueueSize;
                }
            }
            return retVal;
        }

        internal bool IsAsduAvailable()
        {
            if (enqueuedASDUs == null)
                return false;

            if (NumberOfAsduInQueue > 0)
            {
                int currentIndex = oldestQueueEntry;

                while (enqueuedASDUs[currentIndex].state != QueueEntryState.WAITING_FOR_TRANSMISSION)
                {
                    if (currentIndex == latestQueueEntry)
                        break;

                    if (enqueuedASDUs[currentIndex].state == QueueEntryState.NOT_USED)
                        break;

                    currentIndex = (currentIndex + 1) % maxQueueSize;

                }

                if (enqueuedASDUs[currentIndex].state == QueueEntryState.WAITING_FOR_TRANSMISSION)
                {
                    enqueuedASDUs[currentIndex].state = QueueEntryState.SENT_BUT_NOT_CONFIRMED;
                    return true;
                }

                return false;
            }

            return false;
        }

        internal bool IsHighPriorityAsduAvailable()
        {
            if (enqueuedASDUs == null)
                return false;

            if (NumberOfAsduInQueue > 0)
            {
                return true;
            }
            else
            {
                return false;
            }
        }

        internal BufferFrame GetNextWaitingASDU(out long timestamp, out int index)
        {
            timestamp = 0;
            index = -1;

            if (enqueuedASDUs == null)
                return null;

            if (NumberOfAsduInQueue > 0)
            {
                int currentIndex = oldestQueueEntry;

                while (enqueuedASDUs[currentIndex].state != QueueEntryState.WAITING_FOR_TRANSMISSION)
                {
                    if (currentIndex == latestQueueEntry)
                        break;

                    if (enqueuedASDUs[currentIndex].state == QueueEntryState.NOT_USED)
                        break;

                    currentIndex = (currentIndex + 1) % maxQueueSize;

                }

                if (enqueuedASDUs[currentIndex].state == QueueEntryState.WAITING_FOR_TRANSMISSION)
                {
                    enqueuedASDUs[currentIndex].state = QueueEntryState.SENT_BUT_NOT_CONFIRMED;
                    timestamp = enqueuedASDUs[currentIndex].entryTimestamp;
                    index = currentIndex;
                    return enqueuedASDUs[currentIndex].asdu;
                }

                return null;
            }

            return null;
        }

        internal BufferFrame GetNextHighPriorityWaitingASDU(out long timestamp, out int index)
        {
            timestamp = 0;
            index = -1;

            if (enqueuedASDUs == null)
                return null;

            if (NumberOfAsduInQueue > 0)
            {
                NumberOfAsduInQueue--;

                int currentIndex = oldestQueueEntry;

                BufferFrame bufferFrame = enqueuedASDUs[currentIndex].asdu;

                timestamp = enqueuedASDUs[currentIndex].entryTimestamp;
                index = currentIndex;

                return bufferFrame;
            }

            return null;
        }

        public void UnmarkAllASDUs()
        {
            lock (enqueuedASDUs)
            {
                if (NumberOfAsduInQueue > 0)
                {
                    for (int i = 0; i < enqueuedASDUs.Length; i++)
                    {
                        if (enqueuedASDUs[i].state == QueueEntryState.SENT_BUT_NOT_CONFIRMED)
                            enqueuedASDUs[i].state = QueueEntryState.WAITING_FOR_TRANSMISSION;
                    }
                }
            }
        }

        public void MarkASDUAsConfirmed(int index, long timestamp)
        {
            if (enqueuedASDUs == null)
                return;

            if ((index < 0) || (index > enqueuedASDUs.Length))
                return;

            lock (enqueuedASDUs)
            {
                if (numberOfAsduInQueue > 0)
                {
                    if (enqueuedASDUs[index].state == QueueEntryState.SENT_BUT_NOT_CONFIRMED)
                    {
                        if (enqueuedASDUs[index].entryTimestamp == timestamp)
                        {
                            int currentIndex = index;

                            while (enqueuedASDUs[currentIndex].state == QueueEntryState.SENT_BUT_NOT_CONFIRMED)
                            {
                                DebugLog("Remove from queue with index " + currentIndex);

                                enqueuedASDUs[currentIndex].state = QueueEntryState.NOT_USED;
                                enqueuedASDUs[currentIndex].entryTimestamp = 0;
                                NumberOfAsduInQueue -= 1;

                                if (NumberOfAsduInQueue == 0)
                                {
                                    oldestQueueEntry = -1;
                                    latestQueueEntry = -1;

                                    break;
                                }

                                if (currentIndex == oldestQueueEntry)
                                {
                                    oldestQueueEntry = (index + 1) % maxQueueSize;

                                    if (NumberOfAsduInQueue == 1)
                                        latestQueueEntry = oldestQueueEntry;

                                    break;
                                }

                                currentIndex = currentIndex - 1;

                                if (currentIndex < 0)
                                    currentIndex = maxQueueSize - 1;

                                if (currentIndex == index)
                                    break;

                            }

                            DebugLog("queue state: noASDUs: " + NumberOfAsduInQueue + " oldest: " + oldestQueueEntry + " latest: " + latestQueueEntry);
                        }
                    }
                }
            }

        }

    }

}
