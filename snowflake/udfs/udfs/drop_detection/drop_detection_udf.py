import datetime
import uuid

import pandas as pd

class Result:
    def __init__(self, job_type, detection_id, endpoint, direction, avg_drop, stdev_drop, anomaly_drop_date, anomaly_drop_number):
        self.job_type = job_type
        if not detection_id:
            self.detection_id = str(uuid.uuid4())
        else:
            self.detection_id = detection_id
        self.time_created = datetime.datetime.now()
        self.endpoint = endpoint
        self.direction = direction
        self.avg_drop = avg_drop
        self.stdev_drop = stdev_drop
        self.anomaly_drop_date = anomaly_drop_date
        self.anomaly_drop_number = anomaly_drop_number

class DropDetection:
    def __init__(self):
        self._date_dropnumber_pairs = []

    def process(self,
                job_type,
                detection_id,
                endpoint,
                direction,
                date,
                drop_number):
        assert(job_type == "initial")
        # ideally this would be done in the constructor, but this is not
        # supported in Snowflake (passing arguments once via the constructor)
        # instead we will keep overriding self._job_type with the same value
        self._job_type = job_type
        self._detection_id = detection_id
        self._endpoint = endpoint
        self._direction = direction
        self._date_dropnumber_pairs.append((date, drop_number))
        yield None

    def end_partition(self):
        # Skip if the amount of data is too small
        if len(self._date_dropnumber_pairs) < 3:
            return
        df = pd.DataFrame(self._date_dropnumber_pairs, columns =['date', 'drop_number'])
        mean = df['drop_number'].mean()
        std = df['drop_number'].std()
        upperbound = mean + 3 * std
        lowerbound = mean - 3 * std
        is_anomaly = (df['drop_number'] > upperbound) | (df['drop_number'] < lowerbound)
        anomalies = df[is_anomaly]
        for date, drop_number in zip(anomalies['date'], anomalies['drop_number']):
            row = Result(self._job_type, self._detection_id, self._endpoint, self._direction, mean, std, date, drop_number)
            yield (row.job_type, row.detection_id, row.time_created, row.endpoint, row.direction, row.avg_drop, row.stdev_drop, row.anomaly_drop_date, row.anomaly_drop_number)
