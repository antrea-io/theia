import datetime
import uuid

import pandas as pd

class Result:
    def __init__(self, job_type, detectionID, endpoint, direction, avgDrop, stdevDrop, anomalyDropDate, anomalyDropNumber):
        self.job_type = job_type
        if not detectionID:
            self.detectionID = str(uuid.uuid4())
        else:
            self.detectionID = detectionID
        self.time_created = datetime.datetime.now()
        self.endpoint = endpoint
        self.direction = direction
        self.avgDrop = avgDrop
        self.stdevDrop = stdevDrop
        self.anomalyDropDate = anomalyDropDate
        self.anomalyDropNumber = anomalyDropNumber

class TrafficDrop:
    def __init__(self):
        self._date_dropnumber_pairs = []

    def process(self,
                jobType,
                detectionID,
                endpoint,
                direction,
                date,
                dropNumber):
        # ideally this would be done in the constructor, but this is not
        # supported in Snowflake (passing arguments once via the constructor)
        # instead we will keep overriding self._jobType with the same value
        self._jobType = jobType
        self._detectionID = detectionID
        self._endpoint = endpoint
        self._direction = direction
        self._date_dropnumber_pairs.append((date, dropNumber))
        yield None

    def end_partition(self):
        # Skip if the amount of data is too small
        if len(self._date_dropnumber_pairs) < 3:
            return
        df = pd.DataFrame(self._date_dropnumber_pairs, columns =['date', 'dropNumber'])
        mean = df['dropNumber'].mean()
        std = df['dropNumber'].std()
        upperbound = mean + 3 * std
        lowerbound = mean - 3 * std
        is_anomaly = (df['dropNumber'] > upperbound) | (df['dropNumber'] < lowerbound)
        anomalies = df[is_anomaly]
        for date, dropNumber in zip(anomalies['date'], anomalies['dropNumber']):
            row = Result(self._jobType, self._detectionID, self._endpoint, self._direction, mean, std, date, dropNumber)
            yield (row.job_type, row.detectionID, row.time_created, row.endpoint, row.direction, row.avgDrop, row.stdevDrop, row.anomalyDropDate, row.anomalyDropNumber)
