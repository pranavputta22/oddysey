import { firebase } from '@react-native-firebase/functions';
import { Representative } from '../../models';
import NetInfo from '@react-native-community/netinfo';
import Axios from 'axios';
import { Bill, BillMetadata } from '../../models/Bill';
import { BillData, Comment, Vote } from '../../models/BillData';
import perf from '@react-native-firebase/perf';
import { User } from '../../redux/models/user';
import messaging from '@react-native-firebase/messaging';
import store from '../../redux/store';
import { StorageService } from '../../redux/storage';
import { Alert } from 'react-native';
import { Committee } from '../../redux/models/committee';

export module Network {
  export async function isNetworkAvailable() {
    const response = await NetInfo.fetch();
    return response.isConnected;
  }

  const awsURLs = {
    userExists:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/user-exists',
    newUser:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/new-user',
    refresh:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/refresh',
    billFeed:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/bill-feed',
    search:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/search',
    like: 'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/like',
    likedBills:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/liked-bills',
    billData:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/get-bill-data',
    vote: 'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/vote',
    addComment: 'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/add-comment',
    likeComment:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/like-comment',
    deleteComment:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/delete-comment',
    uploadPFP:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/upload-pfp',
    updateProfile:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/update-profile',
    deleteUser:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/delete-user',
    emailRep:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/email-rep',
    fetchBill:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/fetch-bill',
    fetchCommittees:
      'https://tde26c6cp5.execute-api.us-east-2.amazonaws.com/prod/fetch-committees',
  };

  export function setupPerfMonitor() {
    Axios.interceptors.request.use(async function (config: any) {
      try {
        const httpMetric = perf().newHttpMetric(config.url || '', 'POST');
        config.metadata = { httpMetric };

        await httpMetric.start();
      } finally {
        return config;
      }
    });
    Axios.interceptors.response.use(
      async function (response: any) {
        try {
          // Request was successful, e.g. HTTP code 200

          const { httpMetric } = response.config.metadata;

          // add any extra metric attributes if needed
          // httpMetric.putAttribute('userId', '12345678');

          httpMetric.setHttpResponseCode(response.status);
          httpMetric.setResponseContentType(response.headers['content-type']);
          await httpMetric.stop();
        } finally {
          return response;
        }
      },
      async function (error) {
        try {
          // Request failed, e.g. HTTP code 500

          const { httpMetric } = error.config.metadata;

          // add any extra metric attributes if needed
          // httpMetric.putAttribute('userId', '12345678');

          httpMetric.setHttpResponseCode(error.response.status);
          httpMetric.setResponseContentType(
            error.response.headers['content-type']
          );
          await httpMetric.stop();
        } finally {
          // Ensure failed requests throw after interception
          return Promise.reject(error);
        }
      }
    );
  }

  /**
   * refreshes all pertinent user data in the background
   */
  export async function refresh(): Promise<
    [User, Representative[]] | undefined
  > {
    if (!isNetworkAvailable()) {
      return undefined;
    }
    return Axios.post(awsURLs.refresh, {
      uid: firebase.auth().currentUser?.uid,
    })
      .then(async (res) => {
        // console.error(res);
        let user: User = res.data.userData;
        let reps = res.data.reps as Representative[];

        return [user, reps] as [User, Representative[]];
      })
      .catch((err) => {
        console.error(err);
        return undefined;
      });
  }

  export async function userExists(uid: string): Promise<boolean> {
    if (!isNetworkAvailable()) {
      return false;
    }

    return await Axios.post(awsURLs.userExists, {
      uid: uid,
    })
      .then((response) => {
        if (response.status == 200 && response.data.result) {
          return true;
        } else {
          return false;
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return false;
      });
  }

  export async function createUser(user: User): Promise<boolean> {
    if (!isNetworkAvailable()) {
      return false;
    }

    let id = await firebase.auth().currentUser?.getIdToken(true);
    console.log(user);
    return Axios.post(awsURLs.newUser, {
      token: id,
      user: user,
    })
      .then((response) => {
        if (response.status == 200 && response.data.result) {
          return true;
        } else {
          return false;
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return false;
      });
  }

  export async function loadBillFeed(): Promise<Bill[]> {
    if (!isNetworkAvailable()) {
      return [];
    }
    let user = StorageService.user();
    return Axios.post(awsURLs.billFeed, { topics: user?.interestedTopics })
      .then((response) => {
        if (response.status == 200) {
          return response.data.bills;
        } else {
          return [];
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return [];
      });
  }

  export async function loadCommittees(): Promise<Committee[]> {
    if (!isNetworkAvailable()) {
      return [];
    }
    return Axios.post(awsURLs.fetchCommittees, {})
      .then((response) => {
        if (response.status == 200) {
          return response.data.committees;
        } else {
          return [];
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return [];
      });
  }

  export async function getBill(
    metadata: BillMetadata
  ): Promise<[Bill, BillData] | undefined> {
    if (!isNetworkAvailable()) {
      return undefined;
    }
    return Axios.post(awsURLs.fetchBill, { bill: metadata })
      .then((response) => {
        if (response.status == 200) {
          return [response.data.bills, response.data.data] as [Bill, BillData];
        } else {
          return undefined;
        }
      })
      .catch((err) => {
        return undefined;
      });
  }

  export async function likedBills(): Promise<Bill[]> {
    if (!isNetworkAvailable()) {
      return [];
    }
    return Axios.post(awsURLs.likedBills, {
      user: StorageService.user(),
    })
      .then((response) => {
        if (response.status == 200) {
          return response.data.bills;
        } else {
          return [];
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return [];
      });
  }

  export async function search(
    searchBy: string,
    query: string
  ): Promise<Bill[]> {
    if (!isNetworkAvailable()) {
      throw 'no internet';
    }
    return Axios.post(awsURLs.search, { searchBy: searchBy, query: query })
      .then((response) => {
        if (response.status == 200) {
          return response.data.bills;
        } else {
          return [];
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return [];
      });
  }

  /**
   * convert bill number to bill identifier
   */
  export async function likeBill(bill: Bill, liked: boolean): Promise<any> {
    if (!isNetworkAvailable()) {
      return [];
    }
    let user = StorageService.user();
    let uid = user.uid;
    let id = bill.assembly + bill.chamber + bill.number;
    if (user) {
      user.liked[id] = liked;
      store.dispatch(StorageService.update({ user }));
    }
    if (liked) {
      await messaging().subscribeToTopic(id);
    } else {
      await messaging().unsubscribeFromTopic(id);
    }
    return Axios.post(awsURLs.like, {
      bill_id: id,
      uid: uid,
      liked: liked,
    })
      .then((response) => {
        if (response.status == 200) {
          return true;
        } else {
          return [];
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return [];
      });
  }

  export async function getBillData(bill: Bill): Promise<BillData | undefined> {
    let id = bill.assembly + bill.chamber + bill.number;
    if (!isNetworkAvailable()) {
      return {
        bill_id: id,
        comments: {},
        votes: {},
      };
    }
    return Axios.post(awsURLs.billData, {
      bill_id: id,
    })
      .then((response) => {
        if (response.status == 200) {
          return response.data.bill;
        } else {
          return {
            bill_id: id,
            comments: [],
            votes: {},
          };
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return undefined;
      });
  }

  export async function setBillVote(bill: Bill, vote: Vote): Promise<any> {
    let id = bill.assembly + bill.chamber + bill.number;
    if (!isNetworkAvailable()) {
      return false;
    }
    let user = StorageService.user();
    let uid = user?.uid;
    return Axios.post(awsURLs.vote, {
      bill_id: id,
      uid: uid,
      vote: vote,
    })
      .then((response) => {
        if (response.status == 200) {
          return true;
        } else {
          return [];
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return [];
      });
  }

  export async function addComment(
    bill: Bill,
    comment: Comment,
    shouldSendReps: boolean
  ): Promise<boolean> {
    if (!isNetworkAvailable()) {
      return false;
    }
    let id = bill.assembly + bill.chamber + bill.number;
    let user = StorageService.user();
    let uid = user?.uid;
    return Axios.post(awsURLs.addComment, {
      bill_id: id,
      uid: uid,
      comment: comment,
      shouldSendReps: shouldSendReps,
    })
      .then((response) => {
        if (response.status == 200) {
          return true;
        } else {
          return false;
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return false;
      });
  }

  export async function likeComment(
    bill: Bill,
    cid: string,
    value: boolean
  ): Promise<boolean> {
    if (!isNetworkAvailable()) {
      return false;
    }
    let id = bill.assembly + bill.chamber + bill.number;
    let user = StorageService.user();
    let uid = user?.uid;
    return Axios.post(awsURLs.likeComment, {
      bill_id: id,
      uid: uid,
      cid: cid,
      liked: value,
    })
      .then((response) => {
        if (response.status == 200) {
          return true;
        } else {
          return false;
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return false;
      });
  }

  export async function uploadPFP(user: User, photo: string) {
    const data = {
      uri: photo,
      text: 'image/jpeg',
      name: `${user.uid}_pfp.jpg`,
    };
    if (!isNetworkAvailable()) {
      return false;
    }

    var body = new FormData();
    body.append('uid', user.uid);
    body.append('pfp', data);
    return Axios.post(awsURLs.uploadPFP, body)
      .then((response) => {
        if (response.status == 200) {
          return true;
        } else {
          return false;
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return false;
      });
  }

  export async function deleteComment(
    bill: Bill,
    cid: string
  ): Promise<boolean> {
    if (!isNetworkAvailable()) {
      return false;
    }
    let id = bill.assembly + bill.chamber + bill.number;

    return Axios.post(awsURLs.deleteComment, {
      cid: cid,
      bill_id: id,
    })
      .then((response) => {
        if (response.status == 200) {
          return true;
        } else {
          return false;
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return false;
      });
  }

  export async function updateProfile(user: User): Promise<boolean> {
    if (!isNetworkAvailable()) {
      return false;
    }
    return Axios.post(awsURLs.updateProfile, {
      uid: user.uid,
      name: user.name,
      address: user.address,
      email: user.email,
    })
      .then((response) => {
        if (response.status == 200) {
          return true;
        } else {
          return false;
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return false;
      });
  }

  export async function deleteUser(): Promise<boolean> {
    if (!isNetworkAvailable()) {
      return false;
    }
    let user = StorageService.user();
    return Axios.post(awsURLs.deleteUser, {
      uid: user?.uid,
    })
      .then((response) => {
        if (response.status == 200) {
          return true;
        } else {
          return false;
        }
      })
      .catch((err) => {
        console.log(JSON.stringify(err));
        return false;
      });
  }

  export async function emailRep(
    rep: Representative,
    text: string
  ): Promise<boolean> {
    throw Error('not yet implemented');
  }
}
